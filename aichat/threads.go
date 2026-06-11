package aichat

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Thread is one persisted conversation: an ordered list of UIMessages plus
// metadata for a thread picker.
type Thread struct {
	ID        string      `json:"id"`
	Title     string      `json:"title"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
	Messages  []UIMessage `json:"messages"`
}

// ThreadStore persists conversations so a client can list past threads and
// resume them. Implementations must be safe for concurrent use. There is no
// silent in-memory fallback: when Options.Threads is nil the thread endpoints
// report 501 rather than pretending to persist (see CLAUDE.md CW-3).
type ThreadStore interface {
	Create(ctx context.Context, title string) (*Thread, error)
	List(ctx context.Context) ([]*Thread, error)
	Get(ctx context.Context, id string) (*Thread, error)
	AppendMessage(ctx context.Context, id string, m UIMessage) error
}

// memThreadStore is an in-process ThreadStore for demos and tests. IDs are
// monotonically assigned so creation order is reproducible without a clock or
// randomness in the id.
type memThreadStore struct {
	mu      sync.Mutex
	seq     int
	threads map[string]*Thread
}

// NewMemThreadStore returns an in-memory ThreadStore. Suitable for demos and
// single-process deployments; swap for a DB-backed store in production.
func NewMemThreadStore() ThreadStore {
	return &memThreadStore{threads: map[string]*Thread{}}
}

func (s *memThreadStore) Create(_ context.Context, title string) (*Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	now := time.Now()
	t := &Thread{
		ID:        fmt.Sprintf("thread-%d", s.seq),
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.threads[t.ID] = t
	return t, nil
}

func (s *memThreadStore) List(_ context.Context) ([]*Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Thread, 0, len(s.threads))
	for _, t := range s.threads {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (s *memThreadStore) Get(_ context.Context, id string) (*Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.threads[id]
	if !ok {
		return nil, fmt.Errorf("thread %q not found", id)
	}
	return t, nil
}

func (s *memThreadStore) AppendMessage(_ context.Context, id string, m UIMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.threads[id]
	if !ok {
		return fmt.Errorf("thread %q not found", id)
	}
	t.Messages = append(t.Messages, m)
	t.UpdatedAt = time.Now()
	return nil
}
