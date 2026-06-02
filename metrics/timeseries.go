// Package metrics provides a generic timeseries store for recording and
// querying (timestamp, value) points behind a thin Timeseries interface.
//
// The root package ships an in-process implementation (NewMemory) with no
// external dependencies, suitable for CLIs, tests, and single-process
// servers. A cross-process, persistent implementation backed by valkey/redis
// sorted sets lives in the sibling submodule github.com/flanksource/clicky/valkey,
// which depends on valkey-go so the root module stays dependency-free.
//
// Both implementations share the same wire format (see codec.go) so a metric
// recorded by one can be queried by the other, and RegisterRoutes serves
// either over HTTP without knowing which backend it holds.
package metrics

import "time"

// Point is a single observation in a timeseries.
type Point struct {
	At    time.Time `json:"at"`
	Value float64   `json:"value"`
}

// RecordRequest records one Point for metric ID. A zero At is resolved to the
// current time by the implementation.
type RecordRequest struct {
	ID    string
	At    time.Time
	Value float64
}

// QueryRequest reads the points for metric ID whose timestamps fall in
// [Since, Until]. The HTTP handler resolves zero bounds to a default window
// (Until = now, Since = now-1h); implementations treat a zero bound as
// unbounded on that side.
type QueryRequest struct {
	ID    string
	Since time.Time
	Until time.Time
}

// Timeseries stores and retrieves timeseries points. Implementations must be
// safe for concurrent use.
type Timeseries interface {
	// Record appends a point. Recording is best-effort instrumentation: an
	// implementation backed by an unavailable store may return an error, but
	// callers are expected to log-and-continue rather than fail the producer.
	Record(req RecordRequest) error

	// Query returns the points in the requested range, ascending by time.
	Query(req QueryRequest) ([]Point, error)
}
