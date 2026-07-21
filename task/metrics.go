package task

import (
	"time"

	"github.com/flanksource/clicky/metrics"
)

var runMetrics = metrics.NewMemory(metrics.MemoryConfig{
	Retention: time.Hour,
	MaxPoints: 2048,
})

// MetricID scopes one metric series to an immutable run id.
func MetricID(runID, name string) string {
	return runID + ":" + name
}

// RecordMetric appends one bounded live metric point for a run.
func RecordMetric(runID, name string, value float64, at time.Time) error {
	return runMetrics.Record(metrics.RecordRequest{ID: MetricID(runID, name), At: at, Value: value})
}

// Metrics exposes the bounded task metric store for embedding HTTP handlers.
func Metrics() metrics.Timeseries {
	return runMetrics
}
