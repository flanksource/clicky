package prompt

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Manager brokers prompts between producers (which Ask and block) and consumers
// (which Resolve by id). It mirrors the approval-registry channel handoff but is
// generic and backed by a Store so the UI can list/stream pending prompts. The
// process-wide Manager (GlobalManager) also serves as the interactive sink the
// clicky root routes TTY-less PromptSelect/PromptText calls to.
type Manager struct {
	store   Store
	mu      sync.Mutex
	pending map[string]chan Answer
	seq     atomic.Uint64
}

// NewManager returns a Manager backed by store (use NewMemory for the default).
func NewManager(store Store) *Manager {
	if store == nil {
		store = NewMemory(MemoryConfig{})
	}
	return &Manager{store: store, pending: make(map[string]chan Answer)}
}

// Ask registers p as a pending prompt and blocks until it is resolved or ctx is
// cancelled. A blank ID is assigned a unique one. The snapshot is always left in a
// terminal state (answered/cancelled/expired) and the pending channel removed
// before returning.
func (m *Manager) Ask(ctx context.Context, p Prompt) (Answer, error) {
	if p.ID == "" {
		p.ID = m.newID()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	ch := make(chan Answer, 1)

	m.mu.Lock()
	if _, exists := m.pending[p.ID]; exists {
		m.mu.Unlock()
		return Answer{}, fmt.Errorf("prompt %q already pending", p.ID)
	}
	m.pending[p.ID] = ch
	m.mu.Unlock()

	if err := m.store.Set(p.snapshot()); err != nil {
		m.clearPending(p.ID)
		return Answer{}, fmt.Errorf("store prompt %q: %w", p.ID, err)
	}

	defer m.clearPending(p.ID)

	select {
	case <-ctx.Done():
		m.finalize(p.ID, StateExpired, Answer{Cancelled: true, At: time.Now()})
		return Answer{}, ctx.Err()
	case ans := <-ch:
		return ans, nil
	}
}

// Resolve delivers an answer to a pending prompt, unblocking its Ask. A non-
// cancelled answer is validated against the prompt's schema first; an invalid
// answer is rejected (and the prompt stays pending) so the producer never receives
// a malformed value.
func (m *Manager) Resolve(id string, ans Answer) error {
	snap, ok := m.store.Get(id)
	if !ok {
		return fmt.Errorf("no prompt %q", id)
	}
	m.mu.Lock()
	ch, pending := m.pending[id]
	m.mu.Unlock()
	if !pending {
		return fmt.Errorf("prompt %q is not pending", id)
	}
	if ans.At.IsZero() {
		ans.At = time.Now()
	}
	state := StateAnswered
	if ans.Cancelled {
		state = StateCancelled
	} else if err := Validate(snap.Schema, ans.Values); err != nil {
		return fmt.Errorf("answer for prompt %q: %w", id, err)
	}
	select {
	case ch <- ans:
		m.finalize(id, state, ans)
		return nil
	default:
		return fmt.Errorf("prompt %q was already resolved", id)
	}
}

// Pending returns the prompt's snapshot if it is still awaiting an answer.
func (m *Manager) Pending(id string) (PromptSnapshot, bool) {
	m.mu.Lock()
	_, pending := m.pending[id]
	m.mu.Unlock()
	if !pending {
		return PromptSnapshot{}, false
	}
	return m.store.Get(id)
}

// Snapshot returns the prompt's current snapshot regardless of state.
func (m *Manager) Snapshot(id string) (PromptSnapshot, bool) { return m.store.Get(id) }

// List returns snapshots matching filter (newest first).
func (m *Manager) List(filter Filter) []PromptSnapshot { return m.store.List(filter) }

func (m *Manager) finalize(id string, state State, ans Answer) {
	snap, ok := m.store.Get(id)
	if !ok {
		return
	}
	snap.State = string(state)
	snap.Cancelled = ans.Cancelled
	if !ans.Cancelled {
		snap.Value = ans.Values
	}
	snap.ResolvedAt = ans.At.UTC().Format(time.RFC3339)
	_ = m.store.Set(snap)
}

func (m *Manager) clearPending(id string) {
	m.mu.Lock()
	delete(m.pending, id)
	m.mu.Unlock()
}

func (m *Manager) newID() string {
	return fmt.Sprintf("prompt-%d-%d", time.Now().UnixNano(), m.seq.Add(1))
}
