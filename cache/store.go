package cache

import (
	"context"
	"strconv"
	"time"
)

// Store is a minimal, Redis-shaped key/value surface: enough string and
// sorted-set commands to back clicky's domain stores (prompt.Store,
// metrics.Timeseries) without exposing a full client. The in-process backend
// (NewMemory) lives here in the dependency-free root module; the valkey-backed
// backend (valkey.NewStore) lives in the sibling submodule. Domain stores are
// written once against this interface and run unchanged on either backend.
//
// Keys are opaque and fully qualified: Store applies no namespacing. Callers
// own key naming (e.g. prompt builds "prefix:prompt:<id>"). Implementations
// must be safe for concurrent use.
type Store interface {
	// Set writes value at key. A ttl <= 0 persists the key with no expiry; a
	// positive ttl sets an expiry, replacing any existing one.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Get returns the value at key, or ErrKeyNotFound if the key is absent or
	// expired. Any other error is transient (backend unavailable, decode, …) and
	// must be distinguishable from a genuine miss — callers prune index entries
	// only on ErrKeyNotFound.
	Get(ctx context.Context, key string) ([]byte, error)
	// Del removes key (of any type). Removing a missing key is not an error.
	Del(ctx context.Context, key string) error
	// Expire sets a ttl on an existing key of any type; it does not change the
	// value. A missing key is a no-op.
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// ZAdd inserts or updates member with score in the sorted set at key. It
	// does not affect any TTL already set on the key.
	ZAdd(ctx context.Context, key string, score float64, member string) error
	// ZRem removes member from the sorted set at key.
	ZRem(ctx context.Context, key, member string) error
	// ZRevRange returns members ranked by score high→low over the inclusive rank
	// range [start, stop]; negative indices count from the end (Redis
	// semantics). Used for newest-first indexes.
	ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	// ZRangeByScore returns members whose score falls within [lower, upper],
	// ascending by score.
	ZRangeByScore(ctx context.Context, key string, lower, upper Bound) ([]string, error)
	// ZRemRangeByScore removes members whose score falls within [lower, upper].
	ZRemRangeByScore(ctx context.Context, key string, lower, upper Bound) error
	// ZRemRangeByRank removes members in the inclusive rank range [start, stop],
	// ranked by score low→high; negative indices count from the end. Trimming a
	// sorted set to its newest N members is ZRemRangeByRank(key, 0, -(N+1)).
	ZRemRangeByRank(ctx context.Context, key string, start, stop int64) error
}

// Bound is one endpoint of a sorted-set score range. It models Redis's three
// forms: inclusive (12), exclusive ("(12") and unbounded (-inf / +inf).
type Bound struct {
	Score     float64
	Inclusive bool
	// Inf selects an unbounded endpoint: -1 for -inf, +1 for +inf, 0 to use
	// Score.
	Inf int8
}

// Inclusive is the closed bound at score s.
func Inclusive(s float64) Bound { return Bound{Score: s, Inclusive: true} }

// Exclusive is the open bound at score s ("(s" in Redis), excluding s itself.
func Exclusive(s float64) Bound { return Bound{Score: s} }

// NegInf and PosInf are the unbounded endpoints, used when a query side is open.
var (
	NegInf = Bound{Inf: -1}
	PosInf = Bound{Inf: 1}
)

// Redis renders the bound as a ZRANGEBYSCORE-family argument: "-inf"/"+inf" for
// the unbounded ends, "s" for an inclusive score, "(s" for an exclusive one. It
// is exported so the valkey adapter in the sibling module renders bounds the
// same way the in-memory backend interprets them.
func (b Bound) Redis() string {
	switch b.Inf {
	case -1:
		return "-inf"
	case 1:
		return "+inf"
	}
	s := strconv.FormatFloat(b.Score, 'f', -1, 64)
	if b.Inclusive {
		return s
	}
	return "(" + s
}

// matchesLower reports whether score satisfies b used as a range's lower bound.
func (b Bound) matchesLower(score float64) bool {
	switch b.Inf {
	case -1:
		return true
	case 1:
		return false
	}
	if b.Inclusive {
		return score >= b.Score
	}
	return score > b.Score
}

// matchesUpper reports whether score satisfies b used as a range's upper bound.
func (b Bound) matchesUpper(score float64) bool {
	switch b.Inf {
	case 1:
		return true
	case -1:
		return false
	}
	if b.Inclusive {
		return score <= b.Score
	}
	return score < b.Score
}
