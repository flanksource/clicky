package metrics

import (
	"sort"
	"sync"
	"time"
)

// MemoryConfig tunes the in-process Timeseries. Zero fields fall back to
// defaults: Retention 1h, MaxPoints 4096.
type MemoryConfig struct {
	// Retention drops points older than this on each Record. Zero -> 1h.
	Retention time.Duration
	// MaxPoints caps the points kept per metric ID; the oldest are dropped
	// first on overflow. Zero -> 4096.
	MaxPoints int
}

const (
	defaultMemoryRetention = time.Hour
	defaultMemoryMaxPoints = 4096
)

// memoryStore is a bounded, in-process Timeseries. Points per ID are kept
// sorted ascending by time so Record's trim and Query's range scan are both
// cheap. It holds no external resources and is safe for concurrent use.
type memoryStore struct {
	mu        sync.Mutex
	series    map[string][]Point
	retention time.Duration
	maxPoints int
}

// NewMemory returns an in-process Timeseries. It needs no backend and is the
// zero-config default for CLIs, tests, and single-process servers.
func NewMemory(cfg MemoryConfig) Timeseries {
	if cfg.Retention <= 0 {
		cfg.Retention = defaultMemoryRetention
	}
	if cfg.MaxPoints <= 0 {
		cfg.MaxPoints = defaultMemoryMaxPoints
	}
	return &memoryStore{
		series:    make(map[string][]Point),
		retention: cfg.Retention,
		maxPoints: cfg.MaxPoints,
	}
}

func (m *memoryStore) Record(req RecordRequest) error {
	at := req.At
	if at.IsZero() {
		at = time.Now()
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	pts := append(m.series[req.ID], Point{At: at, Value: req.Value})
	// Keep ascending by time; the appended point is usually already the
	// latest, so this is a near-no-op in the common path.
	sort.Slice(pts, func(i, j int) bool { return pts[i].At.Before(pts[j].At) })

	cutoff := at.Add(-m.retention)
	pts = dropBefore(pts, cutoff)
	if len(pts) > m.maxPoints {
		pts = pts[len(pts)-m.maxPoints:]
	}
	m.series[req.ID] = pts
	return nil
}

func (m *memoryStore) Query(req QueryRequest) ([]Point, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	src := m.series[req.ID]
	out := make([]Point, 0, len(src))
	for _, p := range src {
		if !req.Since.IsZero() && p.At.Before(req.Since) {
			continue
		}
		if !req.Until.IsZero() && p.At.After(req.Until) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// dropBefore returns the suffix of pts (sorted ascending) whose timestamps are
// at or after cutoff.
func dropBefore(pts []Point, cutoff time.Time) []Point {
	i := sort.Search(len(pts), func(i int) bool { return !pts[i].At.Before(cutoff) })
	return pts[i:]
}
