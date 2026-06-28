package prompt

import (
	"sort"
	"sync"
	"time"
)

// Store persists prompt snapshots so the manager's state survives a UI reconnect
// and can (with a valkey-backed Store) be shared across processes. It mirrors the
// metrics.Timeseries split: NewMemory here, valkey.New in the submodule.
// Implementations must be safe for concurrent use.
type Store interface {
	Set(snap PromptSnapshot) error
	Get(id string) (PromptSnapshot, bool)
	Delete(id string) error
	// List returns snapshots matching filter, newest first.
	List(filter Filter) []PromptSnapshot
}

// MemoryConfig tunes the in-process Store. Zero Retention -> 10m: resolved
// prompts are kept that long so a reconnecting UI still sees the outcome, then
// GC'd. Pending prompts are never GC'd.
type MemoryConfig struct {
	Retention time.Duration
}

const defaultMemoryRetention = 10 * time.Minute

type memoryStore struct {
	mu        sync.Mutex
	byID      map[string]PromptSnapshot
	retention time.Duration
}

// NewMemory returns an in-process Store. It needs no backend and is the
// zero-config default for CLIs, tests, and single-process servers.
func NewMemory(cfg MemoryConfig) Store {
	if cfg.Retention <= 0 {
		cfg.Retention = defaultMemoryRetention
	}
	return &memoryStore{
		byID:      make(map[string]PromptSnapshot),
		retention: cfg.Retention,
	}
}

func (m *memoryStore) Set(snap PromptSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gcLocked()
	m.byID[snap.ID] = snap
	return nil
}

func (m *memoryStore) Get(id string) (PromptSnapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gcLocked()
	s, ok := m.byID[id]
	return s, ok
}

func (m *memoryStore) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.byID, id)
	return nil
}

func (m *memoryStore) List(filter Filter) []PromptSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gcLocked()
	out := make([]PromptSnapshot, 0, len(m.byID))
	for _, s := range m.byID {
		if filter.matches(s) {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

// gcLocked drops resolved snapshots older than the retention window. Pending
// prompts (no ResolvedAt) are kept until they resolve. Caller holds m.mu.
func (m *memoryStore) gcLocked() {
	cutoff := time.Now().Add(-m.retention)
	for id, s := range m.byID {
		if s.ResolvedAt == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, s.ResolvedAt); err == nil && t.Before(cutoff) {
			delete(m.byID, id)
		}
	}
}
