// Package valkey backs clicky's cache.Store (and cache.Browser) with a
// valkey/redis server. It lives in a separate Go module so the valkey-go
// dependency stays out of the root clicky module: callers that only need the
// in-process stores (cache.NewMemory) never pull it in.
//
// NewStore returns a cache.Store adapter over a valkey.Client; the domain stores
// compose over it, e.g. prompt.NewStore(valkey.NewStore(client), cfg) and
// metrics.NewStore(valkey.NewStore(client), cfg). One adapter (and one client)
// can back the prompt store, the metrics store, and the cache browser at once.
package valkey

import (
	"context"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/flanksource/clicky/cache"
)

// opTimeout caps any single round-trip so a store operation never stalls the
// caller. Mirrors the timeout the cache browser applies to its single-key ops.
const opTimeout = 250 * time.Millisecond

type kvStore struct {
	client    valkey.Client
	opTimeout time.Duration
}

// NewStore returns a cache.Store backed by client. The client is owned by the
// caller (NewStore does not close it), so an app can share its existing
// connection rather than opening a second one.
func NewStore(client valkey.Client) cache.Store {
	return &kvStore{client: client, opTimeout: opTimeout}
}

func (s *kvStore) ctx(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, s.opTimeout)
}

func (s *kvStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	set := s.client.B().Set().Key(key).Value(string(value))
	var cmd valkey.Completed
	if ttl > 0 {
		cmd = set.PxMilliseconds(atLeastOneMilli(ttl)).Build()
	} else {
		cmd = set.Build()
	}
	return s.client.Do(ctx, cmd).Error()
}

func (s *kvStore) Get(ctx context.Context, key string) ([]byte, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	data, err := s.client.Do(ctx, s.client.B().Get().Key(key).Build()).AsBytes()
	if valkey.IsValkeyNil(err) {
		return nil, cache.ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *kvStore) Del(ctx context.Context, key string) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	return s.client.Do(ctx, s.client.B().Del().Key(key).Build()).Error()
}

func (s *kvStore) Expire(ctx context.Context, key string, ttl time.Duration) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	// ttl <= 0 removes the expiry (PERSIST), matching the in-memory store; a 1ms
	// PEXPIRE would instead reap the key almost immediately.
	if ttl <= 0 {
		return s.client.Do(ctx, s.client.B().Persist().Key(key).Build()).Error()
	}
	return s.client.Do(ctx, s.client.B().Pexpire().Key(key).Milliseconds(atLeastOneMilli(ttl)).Build()).Error()
}

func (s *kvStore) ZAdd(ctx context.Context, key string, score float64, member string) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	return s.client.Do(ctx, s.client.B().Zadd().Key(key).ScoreMember().ScoreMember(score, member).Build()).Error()
}

func (s *kvStore) ZRem(ctx context.Context, key, member string) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	return s.client.Do(ctx, s.client.B().Zrem().Key(key).Member(member).Build()).Error()
}

func (s *kvStore) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	return s.client.Do(ctx, s.client.B().Zrevrange().Key(key).Start(start).Stop(stop).Build()).AsStrSlice()
}

func (s *kvStore) ZRangeByScore(ctx context.Context, key string, lower, upper cache.Bound) ([]string, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	return s.client.Do(ctx, s.client.B().Zrangebyscore().Key(key).
		Min(lower.Redis()).Max(upper.Redis()).Build()).AsStrSlice()
}

func (s *kvStore) ZRemRangeByScore(ctx context.Context, key string, lower, upper cache.Bound) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	return s.client.Do(ctx, s.client.B().Zremrangebyscore().Key(key).
		Min(lower.Redis()).Max(upper.Redis()).Build()).Error()
}

func (s *kvStore) ZRemRangeByRank(ctx context.Context, key string, start, stop int64) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	return s.client.Do(ctx, s.client.B().Zremrangebyrank().Key(key).Start(start).Stop(stop).Build()).Error()
}

// atLeastOneMilli renders a positive ttl in whole milliseconds, never rounding a
// genuinely-positive duration down to 0 (which Redis rejects as an invalid
// expiry). Callers reach this path only with ttl > 0.
func atLeastOneMilli(ttl time.Duration) int64 {
	if ms := ttl.Milliseconds(); ms > 0 {
		return ms
	}
	return 1
}
