package metrics

import (
	"context"
	"time"

	"github.com/flanksource/clicky/cache"
)

// StoreConfig tunes a cache-backed Timeseries.
type StoreConfig struct {
	// KeyPrefix is prepended to every key so metrics share the caller's
	// namespace. The full key is KeyPrefix + "metric:" + id.
	KeyPrefix string
	// Retention drops points older than this on each Record. Zero -> 7d.
	Retention time.Duration
	// MaxPoints caps the points kept per metric ID, dropping the oldest on
	// overflow. Zero leaves the series uncapped (time-based retention only).
	MaxPoints int
}

// MemoryConfig tunes the in-process Timeseries. Zero fields fall back to
// defaults: Retention 1h, MaxPoints 4096.
type MemoryConfig struct {
	Retention time.Duration
	MaxPoints int
}

const (
	defaultRetention       = 7 * 24 * time.Hour
	defaultMemoryRetention = time.Hour
	defaultMemoryMaxPoints = 4096
)

// NewMemory returns an in-process Timeseries. It needs no backend and is the
// zero-config default for CLIs, tests, and single-process servers. The series is
// bounded (MaxPoints) so a long-running process cannot grow it without limit.
func NewMemory(cfg MemoryConfig) Timeseries {
	if cfg.Retention <= 0 {
		cfg.Retention = defaultMemoryRetention
	}
	if cfg.MaxPoints <= 0 {
		cfg.MaxPoints = defaultMemoryMaxPoints
	}
	return NewStore(cache.NewMemory(), StoreConfig{Retention: cfg.Retention, MaxPoints: cfg.MaxPoints})
}

// NewStore returns a Timeseries backed by kv: one sorted set per metric ID, each
// point a "<unixMillis>:<value>" member scored by its millisecond timestamp (see
// EncodeMember). Pass valkey.NewStore for a cross-process, persistent series. The
// kv backend is owned by the caller.
func NewStore(kv cache.Store, cfg StoreConfig) Timeseries {
	if cfg.Retention <= 0 {
		cfg.Retention = defaultRetention
	}
	if cfg.MaxPoints < 0 {
		cfg.MaxPoints = 0
	}
	return &store{kv: kv, prefix: cfg.KeyPrefix, retention: cfg.Retention, maxPoints: cfg.MaxPoints}
}

type store struct {
	kv        cache.Store
	prefix    string
	retention time.Duration
	maxPoints int
}

func (s *store) key(id string) string { return s.prefix + "metric:" + id }

func (s *store) Record(req RecordRequest) error {
	at := req.At
	if at.IsZero() {
		at = time.Now()
	}
	ctx := context.Background()
	key := s.key(req.ID)

	if err := s.kv.ZAdd(ctx, key, float64(at.UnixMilli()), EncodeMember(Point{At: at, Value: req.Value})); err != nil {
		return err
	}
	// Retention by time, anchored to the recorded point: an exclusive upper bound
	// keeps a point recorded exactly at the cutoff edge.
	cutoff := float64(at.Add(-s.retention).UnixMilli())
	if err := s.kv.ZRemRangeByScore(ctx, key, cache.NegInf, cache.Exclusive(cutoff)); err != nil {
		return err
	}
	// Retention by count: keep only the newest maxPoints (rank 0 is the oldest).
	if s.maxPoints > 0 {
		if err := s.kv.ZRemRangeByRank(ctx, key, 0, -int64(s.maxPoints)-1); err != nil {
			return err
		}
	}
	// Whole-key TTL backstop so a series that stops receiving points self-reaps.
	return s.kv.Expire(ctx, key, s.retention)
}

func (s *store) Query(req QueryRequest) ([]Point, error) {
	lower, upper := cache.NegInf, cache.PosInf
	if !req.Since.IsZero() {
		lower = cache.Inclusive(float64(req.Since.UnixMilli()))
	}
	if !req.Until.IsZero() {
		upper = cache.Inclusive(float64(req.Until.UnixMilli()))
	}
	members, err := s.kv.ZRangeByScore(context.Background(), s.key(req.ID), lower, upper)
	if err != nil {
		return nil, err
	}
	out := make([]Point, 0, len(members))
	for _, m := range members {
		p, err := ParseMember(m)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}
