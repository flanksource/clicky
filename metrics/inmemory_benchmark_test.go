package metrics_test

import (
	"testing"
	"time"

	"github.com/flanksource/clicky/metrics"
)

func BenchmarkMemoryRecordInOrderSingleSeries(b *testing.B) {
	ts := metrics.NewMemory(metrics.MemoryConfig{
		Retention: 365 * 24 * time.Hour,
		MaxPoints: 1_000_000,
	})
	start := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)

	i := 0
	for b.Loop() {
		if err := ts.Record(metrics.RecordRequest{
			ID:    "cpu",
			At:    start.Add(time.Duration(i) * time.Millisecond),
			Value: float64(i),
		}); err != nil {
			b.Fatal(err)
		}
		i++
	}
}
