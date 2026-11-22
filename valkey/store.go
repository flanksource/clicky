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
	"fmt"
	"iter"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/flanksource/clicky/cache"
)

const (
	// opTimeout caps a round-trip that carries no payload to speak of, so a
	// store operation never stalls the caller; MGet and Set grow it. Mirrors the
	// timeout the cache browser applies to its single-key ops.
	opTimeout = 250 * time.Millisecond

	// mgetBatch is how many keys one MGet round trip reads, which bounds what a
	// reader holds.
	mgetBatch = 100

	// perKeyTimeout is what each key adds to a batch read's first budget.
	perKeyTimeout = 10 * time.Millisecond

	// perMBTimeout is what each megabyte adds to a write's budget: a shared L2 a
	// network hop away sustains 0.5-1 MB/s.
	perMBTimeout = 2 * time.Second

	// payloadTimeout bounds the budgets that grow with the payload, so a wedged
	// server still fails. It is all the attempts of one batch read together.
	payloadTimeout = 30 * time.Second
)

type kvStore struct {
	client         valkey.Client
	opTimeout      time.Duration
	payloadTimeout time.Duration
}

// NewStore returns a cache.Store backed by client. The client is owned by the
// caller (NewStore does not close it), so an app can share its existing
// connection rather than opening a second one.
func NewStore(client valkey.Client) cache.Store {
	return &kvStore{client: client, opTimeout: opTimeout, payloadTimeout: payloadTimeout}
}

func (s *kvStore) ctx(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, s.opTimeout)
}

func (s *kvStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, s.setTimeout(len(value)))
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

// setTimeout is the budget of writing size bytes.
func (s *kvStore) setTimeout(size int) time.Duration {
	return min(s.opTimeout+time.Duration(float64(size)/(1<<20)*float64(perMBTimeout)), s.payloadTimeout)
}

// Get reads one small value inside opTimeout. Read a value of megabytes with
// MGet, whose budget grows.
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

// MGet pulls up to mgetBatch keys at a time and reads each batch in one round
// trip — one MGET, or one per hash slot on a cluster — before yielding it. How
// much a batch holds is only known once it arrives, so the budget adapts: a
// batch that outlives it is read again with double, and later batches keep it.
func (s *kvStore) MGet(ctx context.Context, keys iter.Seq[string]) iter.Seq2[cache.Entry, error] {
	return func(yield func(cache.Entry, error) bool) {
		batch := make([]string, 0, mgetBatch)
		var budget time.Duration
		flush := func() bool {
			entries, err := s.mgetAdaptive(ctx, batch, &budget)
			batch = batch[:0]
			if err != nil {
				yield(cache.Entry{}, err)
				return false
			}
			for _, entry := range entries {
				if !yield(entry, nil) {
					return false
				}
			}
			return true
		}
		for key := range keys {
			if batch = append(batch, key); len(batch) == mgetBatch && !flush() {
				return
			}
		}
		if len(batch) > 0 {
			flush()
		}
	}
}

// mgetAdaptive reads keys within budget, at least what their number needs,
// doubling it whenever it expires until the attempts spent payloadTimeout.
func (s *kvStore) mgetAdaptive(parent context.Context, keys []string, budget *time.Duration) ([]cache.Entry, error) {
	*budget = max(*budget, s.opTimeout+time.Duration(len(keys))*perKeyTimeout)
	started := time.Now()
	for {
		ctx, cancel := context.WithTimeout(parent, min(*budget, s.payloadTimeout-time.Since(started)))
		entries, err := s.mget(ctx, keys)
		expired := ctx.Err() != nil
		cancel()
		if err == nil || !expired || parent.Err() != nil {
			return entries, err
		}
		if spent := time.Since(started); spent >= s.payloadTimeout {
			return nil, fmt.Errorf("after %s: %w", spent.Round(time.Millisecond), err)
		}
		*budget *= 2
	}
}

func (s *kvStore) mget(ctx context.Context, keys []string) ([]cache.Entry, error) {
	replies, err := valkey.MGet(s.client, ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("mget %d keys: %w", len(keys), err)
	}
	entries := make([]cache.Entry, len(keys))
	for index, key := range keys {
		reply, ok := replies[key]
		if !ok {
			return nil, fmt.Errorf("mget: no reply for key %q", key)
		}
		value, err := reply.AsBytes()
		switch {
		case valkey.IsValkeyNil(err):
			entries[index] = cache.Entry{Key: key}
		case err != nil:
			return nil, fmt.Errorf("mget %q: %w", key, err)
		default:
			entries[index] = cache.Entry{Key: key, Value: value, Found: true}
		}
	}
	return entries, nil
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
