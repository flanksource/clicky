package rpchttp

import (
	"net/http"
	"time"
)

// TimingMiddleware installs a request-scoped Timings accumulator into the
// request context and stamps a Server-Timing response header. The header is
// written on the first WriteHeader/Write, or — for handlers that return without
// writing anything — once the handler returns. It carries a `total` metric plus
// any phase metrics business logic contributed via AddTiming/Track.
func TimingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, timings := WithTimings(r.Context())
		rec := &timingRecorder{ResponseWriter: w, timings: timings, start: time.Now()}
		next.ServeHTTP(rec, r.WithContext(ctx))
		// Handlers that return without writing never trigger the write-path
		// stamp; do it here so the header is emitted regardless. stamp() is
		// idempotent, so a handler that already wrote is unaffected.
		rec.stamp()
	})
}

// timingRecorder stamps the Server-Timing header exactly once, before the header
// block flushes, so it coexists with headers the inner handler already set.
type timingRecorder struct {
	http.ResponseWriter
	timings *Timings
	start   time.Time
	stamped bool
}

func (rec *timingRecorder) stamp() {
	if rec.stamped {
		return
	}
	rec.stamped = true
	value := formatMetric("total", time.Since(rec.start), "")
	if phases := rec.timings.Header(); phases != "" {
		value += ", " + phases
	}
	rec.ResponseWriter.Header().Set("Server-Timing", value)
}

func (rec *timingRecorder) WriteHeader(status int) {
	rec.stamp()
	rec.ResponseWriter.WriteHeader(status)
}

func (rec *timingRecorder) Write(b []byte) (int, error) {
	rec.stamp()
	return rec.ResponseWriter.Write(b)
}

// Flush forwards to the inner Flusher so SSE handlers that assert
// http.Flusher keep streaming.
func (rec *timingRecorder) Flush() {
	rec.stamp()
	if f, ok := rec.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap exposes the wrapped writer to http.ResponseController.
func (rec *timingRecorder) Unwrap() http.ResponseWriter {
	return rec.ResponseWriter
}
