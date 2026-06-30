package cache

import (
	"context"
	"sort"
	"sync"
	"time"
)

// memory is an in-process Store: a hand-written, dependency-free stand-in for
// the subset of valkey/redis the domain stores use (strings with TTL and
// sorted sets). It is the zero-config default for CLIs, tests, and
// single-process servers. Safe for concurrent use behind a single mutex; the
// keyspaces it backs (a prompt index, per-metric series) are small enough that
// sorting sorted sets on demand is cheap.
type memory struct {
	mu      sync.Mutex
	strings map[string][]byte
	zsets   map[string]map[string]float64
	// expiry holds the absolute expiry for any keyed string or sorted set; a key
	// absent here never expires. Expiry is enforced lazily on access.
	expiry map[string]time.Time
}

// NewMemory returns an in-process Store. It needs no backend and holds no
// external resources.
func NewMemory() Store {
	return &memory{
		strings: make(map[string][]byte),
		zsets:   make(map[string]map[string]float64),
		expiry:  make(map[string]time.Time),
	}
}

// dropIfExpired removes key from every map and returns true when its TTL has
// passed. Caller holds m.mu. This is the single point where lazy expiry is
// applied, so every command observes a consistent live keyspace.
func (m *memory) dropIfExpired(key string) bool {
	exp, ok := m.expiry[key]
	if !ok {
		return false
	}
	if time.Now().Before(exp) {
		return false
	}
	delete(m.strings, key)
	delete(m.zsets, key)
	delete(m.expiry, key)
	return true
}

func (m *memory) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.strings[key] = append([]byte(nil), value...)
	delete(m.zsets, key)
	if ttl > 0 {
		m.expiry[key] = time.Now().Add(ttl)
	} else {
		delete(m.expiry, key)
	}
	return nil
}

func (m *memory) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dropIfExpired(key) {
		return nil, ErrKeyNotFound
	}
	v, ok := m.strings[key]
	if !ok {
		return nil, ErrKeyNotFound
	}
	return append([]byte(nil), v...), nil
}

func (m *memory) Del(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.strings, key)
	delete(m.zsets, key)
	delete(m.expiry, key)
	return nil
}

func (m *memory) Expire(_ context.Context, key string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dropIfExpired(key) {
		return nil
	}
	_, isStr := m.strings[key]
	_, isZ := m.zsets[key]
	if !isStr && !isZ {
		return nil
	}
	if ttl > 0 {
		m.expiry[key] = time.Now().Add(ttl)
	} else {
		delete(m.expiry, key)
	}
	return nil
}

func (m *memory) ZAdd(_ context.Context, key string, score float64, member string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dropIfExpired(key)
	// A key is a single type: promoting it to a sorted set clears any string value
	// (mirrors Set, which clears the zset), so a stale string can't leak via Get.
	delete(m.strings, key)
	z := m.zsets[key]
	if z == nil {
		z = make(map[string]float64)
		m.zsets[key] = z
	}
	z[member] = score
	return nil
}

func (m *memory) ZRem(_ context.Context, key, member string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dropIfExpired(key)
	m.removeMembers(key, member)
	return nil
}

func (m *memory) ZRevRange(_ context.Context, key string, start, stop int64) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dropIfExpired(key)
	all := sortedMembers(m.zsets[key], true)
	lo, hi := normRank(start, stop, int64(len(all)))
	out := make([]string, 0, hi-lo)
	for _, s := range all[lo:hi] {
		out = append(out, s.member)
	}
	return out, nil
}

func (m *memory) ZRangeByScore(_ context.Context, key string, lower, upper Bound) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dropIfExpired(key)
	all := sortedMembers(m.zsets[key], false)
	out := make([]string, 0, len(all))
	for _, s := range all {
		if lower.matchesLower(s.score) && upper.matchesUpper(s.score) {
			out = append(out, s.member)
		}
	}
	return out, nil
}

func (m *memory) ZRemRangeByScore(_ context.Context, key string, lower, upper Bound) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dropIfExpired(key)
	z := m.zsets[key]
	var doomed []string
	for member, score := range z {
		if lower.matchesLower(score) && upper.matchesUpper(score) {
			doomed = append(doomed, member)
		}
	}
	m.removeMembers(key, doomed...)
	return nil
}

func (m *memory) ZRemRangeByRank(_ context.Context, key string, start, stop int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dropIfExpired(key)
	z := m.zsets[key]
	// Resolve the rank range from the count alone before paying to sort, so the
	// common "nothing to trim" case (a capped series under its cap) stays O(1).
	lo, hi := normRank(start, stop, int64(len(z)))
	if lo >= hi {
		return nil
	}
	all := sortedMembers(z, false)
	doomed := make([]string, 0, hi-lo)
	for _, s := range all[lo:hi] {
		doomed = append(doomed, s.member)
	}
	m.removeMembers(key, doomed...)
	return nil
}

// removeMembers deletes members from the sorted set at key and drops the set
// (and its TTL) once empty, matching Redis's collapse of empty keys. Caller
// holds m.mu.
func (m *memory) removeMembers(key string, members ...string) {
	z := m.zsets[key]
	if z == nil {
		return
	}
	for _, member := range members {
		delete(z, member)
	}
	if len(z) == 0 {
		delete(m.zsets, key)
		delete(m.expiry, key)
	}
}

type scoredMember struct {
	member string
	score  float64
}

// sortedMembers snapshots a sorted set into a slice ordered by score, breaking
// ties on member name. desc gives ZREVRANGE order (score high→low, member
// reverse-lexicographic); !desc gives ZRANGE/rank order, matching Redis so the
// in-memory and valkey backends agree on equal-score ordering.
func sortedMembers(z map[string]float64, desc bool) []scoredMember {
	out := make([]scoredMember, 0, len(z))
	for member, score := range z {
		out = append(out, scoredMember{member: member, score: score})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			if desc {
				return out[i].score > out[j].score
			}
			return out[i].score < out[j].score
		}
		if desc {
			return out[i].member > out[j].member
		}
		return out[i].member < out[j].member
	})
	return out
}

// normRank resolves a Redis inclusive rank range [start, stop] over n elements
// into a half-open [lo, hi) slice range, applying negative-from-end indexing
// and clamping. It returns lo == hi (empty) when the range selects nothing.
func normRank(start, stop, n int64) (int64, int64) {
	if n == 0 {
		return 0, 0
	}
	if start < 0 {
		start += n
	}
	if stop < 0 {
		stop += n
	}
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	if start > stop {
		return 0, 0
	}
	return start, stop + 1
}
