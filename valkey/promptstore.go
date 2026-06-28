package valkey

import (
	"context"
	"encoding/json"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/flanksource/clicky/prompt"
)

// PromptStoreConfig tunes the valkey-backed prompt.Store.
type PromptStoreConfig struct {
	// KeyPrefix namespaces every key: KeyPrefix + "prompt:" + id for the snapshot
	// and KeyPrefix + "prompts:idx" for the id index.
	KeyPrefix string
	// Retention sets the TTL applied to a prompt once it is resolved, so the UI can
	// still read the outcome on reconnect before it is reaped. Pending prompts never
	// expire. Zero -> 10m (matches the in-memory default).
	Retention time.Duration
}

const defaultPromptRetention = 10 * time.Minute

type promptStore struct {
	client    valkey.Client
	keyPrefix string
	retention time.Duration
}

// NewPromptStore returns a prompt.Store backed by client. The client is owned by
// the caller (NewPromptStore does not close it). Swapping prompt.NewMemory for this
// makes pending prompts and their answers visible across processes sharing a
// valkey instance.
func NewPromptStore(client valkey.Client, cfg PromptStoreConfig) prompt.Store {
	if cfg.Retention <= 0 {
		cfg.Retention = defaultPromptRetention
	}
	return &promptStore{client: client, keyPrefix: cfg.KeyPrefix, retention: cfg.Retention}
}

func (s *promptStore) key(id string) string { return s.keyPrefix + "prompt:" + id }
func (s *promptStore) idxKey() string       { return s.keyPrefix + "prompts:idx" }

func (s *promptStore) Set(snap prompt.PromptSnapshot) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	score := float64(createdScore(snap))
	cmds := valkey.Commands{
		s.client.B().Set().Key(s.key(snap.ID)).Value(string(data)).Build(),
		s.client.B().Zadd().Key(s.idxKey()).ScoreMember().ScoreMember(score, snap.ID).Build(),
	}
	// A resolved prompt is short-lived: TTL it so the outcome lingers for a
	// reconnecting UI then self-reaps. Pending prompts keep no TTL.
	if snap.ResolvedAt != "" {
		cmds = append(cmds, s.client.B().Expire().Key(s.key(snap.ID)).Seconds(int64(s.retention.Seconds())).Build())
	}
	for _, resp := range s.client.DoMulti(ctx, cmds...) {
		if err := resp.Error(); err != nil {
			return err
		}
	}
	return nil
}

func (s *promptStore) Get(id string) (prompt.PromptSnapshot, bool) {
	snap, missing, err := s.getSnapshot(id)
	if err != nil || missing {
		return prompt.PromptSnapshot{}, false
	}
	return snap, true
}

// getSnapshot distinguishes a genuinely-absent key (missing=true, err=nil) from a
// transient read/decode failure (err!=nil). List relies on this so a temporary
// Valkey hiccup never prunes a still-live prompt from the index — only a key the
// backend explicitly reports as gone is pruned.
func (s *promptStore) getSnapshot(id string) (prompt.PromptSnapshot, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	data, err := s.client.Do(ctx, s.client.B().Get().Key(s.key(id)).Build()).AsBytes()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return prompt.PromptSnapshot{}, true, nil
		}
		return prompt.PromptSnapshot{}, false, err
	}
	var snap prompt.PromptSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return prompt.PromptSnapshot{}, false, err
	}
	return snap, false, nil
}

func (s *promptStore) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	cmds := valkey.Commands{
		s.client.B().Del().Key(s.key(id)).Build(),
		s.client.B().Zrem().Key(s.idxKey()).Member(id).Build(),
	}
	for _, resp := range s.client.DoMulti(ctx, cmds...) {
		if err := resp.Error(); err != nil {
			return err
		}
	}
	return nil
}

func (s *promptStore) List(filter prompt.Filter) []prompt.PromptSnapshot {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	// Newest first: highest score (createdAt) first.
	ids, err := s.client.Do(ctx, s.client.B().Zrevrange().Key(s.idxKey()).Start(0).Stop(-1).Build()).AsStrSlice()
	if err != nil || len(ids) == 0 {
		return nil
	}
	out := make([]prompt.PromptSnapshot, 0, len(ids))
	var stale []string
	for _, id := range ids {
		snap, missing, err := s.getSnapshot(id)
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
		if filterMatches(filter, snap) {
			out = append(out, snap)
		}
	}
	for _, id := range stale {
		_ = s.client.Do(ctx, s.client.B().Zrem().Key(s.idxKey()).Member(id).Build()).Error()
	}
	return out
}

func createdScore(snap prompt.PromptSnapshot) int64 {
	if snap.CreatedAt == "" {
		return time.Now().UnixNano()
	}
	if t, err := time.Parse(time.RFC3339, snap.CreatedAt); err == nil {
		return t.UnixNano()
	}
	return time.Now().UnixNano()
}

// filterMatches mirrors prompt.Filter semantics without exporting the unexported
// method: empty fields match anything, all Labels must match.
func filterMatches(f prompt.Filter, s prompt.PromptSnapshot) bool {
	if f.Owner != "" && s.Owner != f.Owner {
		return false
	}
	if f.Kind != "" && s.Kind != f.Kind {
		return false
	}
	if f.State != "" && s.State != f.State {
		return false
	}
	for k, v := range f.Labels {
		got, ok := s.Labels[k]
		if !ok || got != v {
			return false
		}
	}
	return true
}
