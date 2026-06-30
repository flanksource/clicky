package aichat

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	captainai "github.com/flanksource/captain/pkg/ai"
)

// AgentProviderFactory builds a captain StreamingProvider for one agent session.
// The default (defaultAgentProviderFactory) goes through captain's pkg/ai
// provider registry; tests inject a fake to avoid spawning real subprocesses.
type AgentProviderFactory func(cfg captainai.Config) (captainai.StreamingProvider, error)

// defaultAgentIdleTTL is how long an idle pooled provider is kept alive before
// it is evicted (and its supervised subprocess closed) on the next activity.
const defaultAgentIdleTTL = 10 * time.Minute

// pooledProvider is one cached agent provider bound to a captain session. Its
// turn mutex serializes turns on that session (agent backends keep a single
// live session/thread), so concurrent turns for the same conversation queue
// while different conversations run in parallel.
type pooledProvider struct {
	turn      sync.Mutex
	provider  captainai.StreamingProvider
	sessionID string // captain agent session id captured from the backend
	lastUsed  time.Time
}

// providerPool caches one supervised agent provider per active session key,
// reusing it across turns (so the live session continues without re-spawning the
// subprocess) and evicting idle entries. It is safe for concurrent use.
type providerPool struct {
	mu      sync.Mutex
	entries map[string]*pooledProvider
	factory AgentProviderFactory
	ttl     time.Duration
	now     func() time.Time // injectable clock for tests
	seq     atomic.Uint64    // pending-key counter (no randomness, resume-safe)
}

func newProviderPool(factory AgentProviderFactory, ttl time.Duration) *providerPool {
	if ttl <= 0 {
		ttl = defaultAgentIdleTTL
	}
	return &providerPool{
		entries: map[string]*pooledProvider{},
		factory: factory,
		ttl:     ttl,
		now:     time.Now,
	}
}

// pendingKey returns a unique temporary key for a first stateless turn, before
// the backend has assigned a session id. acquire under this key, then rekey to
// the captain session id once the turn yields one.
func (p *providerPool) pendingKey() string {
	return fmt.Sprintf("pending:%d", p.seq.Add(1))
}

// acquire returns the pooled provider for key, creating it via the factory on a
// miss. It first evicts idle entries. The caller MUST hold the returned entry's
// turn mutex for the duration of the turn.
func (p *providerPool) acquire(key string, cfg captainai.Config) (*pooledProvider, error) {
	p.evictIdle()

	p.mu.Lock()
	if e, ok := p.entries[key]; ok {
		e.lastUsed = p.now()
		p.mu.Unlock()
		return e, nil
	}
	p.mu.Unlock()

	// Build the provider without holding p.mu: the factory may spawn a supervised
	// subprocess, and blocking the pool lock for that long would stall every other
	// session's acquire/evict/rekey.
	prov, err := p.factory(cfg)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// A concurrent acquire for the same key may have created the entry while we
	// built ours; prefer the cached one and discard the duplicate provider.
	if e, ok := p.entries[key]; ok {
		closeProvider(prov)
		e.lastUsed = p.now()
		return e, nil
	}
	e := &pooledProvider{provider: prov, lastUsed: p.now()}
	p.entries[key] = e
	return e, nil
}

// touch resets an entry's idle timer to now. A turn holds only entry.turn, so the
// update must go through p.mu to stay synchronized with evictIdle's lastUsed read.
func (p *providerPool) touch(e *pooledProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e.lastUsed = p.now()
}

// rekey moves an entry from oldKey to newKey (e.g. a pending key → the captain
// session id once known). A no-op when oldKey is absent or already at newKey.
func (p *providerPool) rekey(oldKey, newKey string) {
	if oldKey == newKey {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.entries[oldKey]
	if !ok {
		return
	}
	delete(p.entries, oldKey)
	p.entries[newKey] = e
}

// evictIdle closes and removes entries idle longer than the TTL. Entries with a
// turn in progress (turn mutex held) are skipped so a long stream is never
// closed out from under itself.
func (p *providerPool) evictIdle() {
	cutoff := p.now().Add(-p.ttl)

	p.mu.Lock()
	var stale []*pooledProvider
	for key, e := range p.entries {
		if !e.lastUsed.Before(cutoff) {
			continue
		}
		if !e.turn.TryLock() {
			continue // a turn is running; keep it
		}
		stale = append(stale, e)
		delete(p.entries, key)
	}
	p.mu.Unlock()

	for _, e := range stale {
		closeProvider(e.provider)
		e.turn.Unlock()
	}
}

// closeAll drains the pool, closing every provider. Used on server shutdown.
func (p *providerPool) closeAll() {
	p.mu.Lock()
	entries := p.entries
	p.entries = map[string]*pooledProvider{}
	p.mu.Unlock()

	for _, e := range entries {
		e.turn.Lock()
		closeProvider(e.provider)
		e.turn.Unlock()
	}
}

// closeProvider closes a provider if it manages resources (the supervised agent
// subprocess); buffered/stateless providers need no teardown.
func closeProvider(prov captainai.StreamingProvider) {
	if c, ok := prov.(io.Closer); ok {
		_ = c.Close()
	}
}
