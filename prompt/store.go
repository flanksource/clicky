package prompt

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/flanksource/clicky/cache"
)

// Store persists prompt snapshots so the manager's state survives a UI reconnect
// and can — with a cross-process cache.Store — be shared across processes. The
// single implementation (NewStore) runs over any cache.Store: NewMemory for the
// in-process default, valkey.NewStore for a shared valkey backend.
// Implementations must be safe for concurrent use.
type Store interface {
	Set(snap PromptSnapshot) error
	Get(id string) (PromptSnapshot, bool)
	Delete(id string) error
	// List returns snapshots matching filter, newest first.
	List(filter Filter) []PromptSnapshot
}

// StoreConfig tunes a cache-backed Store.
type StoreConfig struct {
	// KeyPrefix namespaces every key: KeyPrefix + "prompt:" + id for a snapshot
	// and KeyPrefix + "prompts:idx" for the id index.
	KeyPrefix string
	// Retention is how long a prompt is kept after it resolves, measured from its
	// ResolvedAt, so a reconnecting UI still reads the outcome before it self-reaps.
	// Pending prompts never expire. Zero -> 10m.
	Retention time.Duration
}

// MemoryConfig tunes the in-process Store. Zero Retention -> 10m.
type MemoryConfig struct {
	Retention time.Duration
}

const defaultRetention = 10 * time.Minute

// NewMemory returns an in-process Store. It needs no backend and is the
// zero-config default for CLIs, tests, and single-process servers.
func NewMemory(cfg MemoryConfig) Store {
	return NewStore(cache.NewMemory(), StoreConfig{Retention: cfg.Retention})
}

// NewStore returns a Store backed by kv. Swapping cache.NewMemory() for a
// valkey-backed cache.Store (valkey.NewStore) makes pending prompts and their
// answers visible across processes sharing one valkey instance. The kv backend
// is owned by the caller.
func NewStore(kv cache.Store, cfg StoreConfig) Store {
	if cfg.Retention <= 0 {
		cfg.Retention = defaultRetention
	}
	return &store{kv: kv, prefix: cfg.KeyPrefix, retention: cfg.Retention}
}

type store struct {
	kv        cache.Store
	prefix    string
	retention time.Duration
}

func (s *store) key(id string) string { return s.prefix + "prompt:" + id }
func (s *store) idxKey() string       { return s.prefix + "prompts:idx" }

func (s *store) Set(snap PromptSnapshot) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	ctx := context.Background()

	// Pending prompts persist (ttl 0); a resolved prompt lives retention-from-its-
	// resolution then self-reaps.
	ttl := time.Duration(0)
	if snap.ResolvedAt != "" {
		ttl = s.resolvedTTL(snap.ResolvedAt)
		if ttl <= 0 {
			// Already past its retention window: ensure it is gone, index and all.
			if err := s.kv.Del(ctx, s.key(snap.ID)); err != nil {
				return err
			}
			return s.kv.ZRem(ctx, s.idxKey(), snap.ID)
		}
	}
	// Index before payload: if the payload write fails the index gains an orphan
	// member, which List() prunes lazily. The reverse order would leave a prompt
	// readable by Get() but invisible to List() — an inconsistency nothing heals.
	if err := s.kv.ZAdd(ctx, s.idxKey(), createdScore(snap), snap.ID); err != nil {
		return err
	}
	return s.kv.Set(ctx, s.key(snap.ID), data, ttl)
}

// resolvedTTL is the time a resolved snapshot should still live: retention
// measured from ResolvedAt. A negative result means it has already outlived the
// window. An unparseable timestamp falls back to the full window (treat as
// just-resolved) rather than reaping a snapshot we cannot date.
func (s *store) resolvedTTL(resolvedAt string) time.Duration {
	t, err := time.Parse(time.RFC3339, resolvedAt)
	if err != nil {
		return s.retention
	}
	return time.Until(t.Add(s.retention))
}

func (s *store) Get(id string) (PromptSnapshot, bool) {
	snap, missing, err := s.getSnapshot(context.Background(), id)
	if err != nil || missing {
		return PromptSnapshot{}, false
	}
	return snap, true
}

// getSnapshot distinguishes a genuinely-absent key (missing=true, err=nil) from a
// transient read/decode failure (err!=nil). List relies on this so a momentary
// backend hiccup never prunes a still-live prompt from the index — only a key the
// backend explicitly reports as gone is pruned.
func (s *store) getSnapshot(ctx context.Context, id string) (PromptSnapshot, bool, error) {
	data, err := s.kv.Get(ctx, s.key(id))
	if errors.Is(err, cache.ErrKeyNotFound) {
		return PromptSnapshot{}, true, nil
	}
	if err != nil {
		return PromptSnapshot{}, false, err
	}
	var snap PromptSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return PromptSnapshot{}, false, err
	}
	return snap, false, nil
}

func (s *store) Delete(id string) error {
	ctx := context.Background()
	if err := s.kv.Del(ctx, s.key(id)); err != nil {
		return err
	}
	return s.kv.ZRem(ctx, s.idxKey(), id)
}

func (s *store) List(filter Filter) []PromptSnapshot {
	ctx := context.Background()
	// Newest first: highest score (createdAt) first.
	ids, err := s.kv.ZRevRange(ctx, s.idxKey(), 0, -1)
	if err != nil || len(ids) == 0 {
		return nil
	}
	out := make([]PromptSnapshot, 0, len(ids))
	var stale []string
	for _, id := range ids {
		snap, missing, err := s.getSnapshot(ctx, id)
		if err != nil {
			// Transient read/decode failure: leave the index member in place so a
			// momentary backend error never permanently drops a live prompt.
			continue
		}
		if missing {
			// The snapshot key expired but its index member lingers; prune it.
			stale = append(stale, id)
			continue
		}
		if filter.matches(snap) {
			out = append(out, snap)
		}
	}
	for _, id := range stale {
		_ = s.kv.ZRem(ctx, s.idxKey(), id)
	}
	return out
}

// createdScore renders a snapshot's creation time as a sorted-set score:
// unix-millis (float64-exact, unlike unix-nanos) so the index orders
// newest-first under ZRevRange. A missing or unparseable timestamp falls back to
// now.
func createdScore(snap PromptSnapshot) float64 {
	if snap.CreatedAt == "" {
		return float64(time.Now().UnixMilli())
	}
	if t, err := time.Parse(time.RFC3339, snap.CreatedAt); err == nil {
		return float64(t.UnixMilli())
	}
	return float64(time.Now().UnixMilli())
}
