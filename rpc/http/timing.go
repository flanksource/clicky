// Package rpchttp provides request-scoped HTTP server timing that both the RPC
// executor path and hand-written handlers can feed, emitted once as a
// Server-Timing response header by TimingMiddleware.
//
// The package deliberately imports only the standard library (not its parent
// rpc package) so business logic can call AddTiming without pulling in rpc's
// cobra/openapi dependencies and without creating an import cycle.
package rpchttp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// timingsKey keys the request-scoped *Timings accumulator into a context.
type timingsKey struct{}

// Timings is a request-scoped, insertion-ordered accumulator of named phase
// durations. Durations for the same name sum, so AddTiming may be called once
// per file in a loop and still report a single total for that phase.
type Timings struct {
	mu     sync.Mutex
	order  []string
	totals map[string]time.Duration
}

// WithTimings returns a context carrying a fresh accumulator and the accumulator
// itself, so a middleware can read it back after the handler returns.
func WithTimings(ctx context.Context) (context.Context, *Timings) {
	t := &Timings{totals: make(map[string]time.Duration)}
	return context.WithValue(ctx, timingsKey{}, t), t
}

// TimingsFromContext returns the accumulator installed by WithTimings, if any.
func TimingsFromContext(ctx context.Context) (*Timings, bool) {
	t, ok := ctx.Value(timingsKey{}).(*Timings)
	return t, ok
}

// AddTiming adds d to the named phase of the current request's accumulator. It
// is a no-op when no accumulator is present (the CLI path has no HTTP response
// to attach a header to).
func AddTiming(ctx context.Context, name string, d time.Duration) {
	if t, ok := TimingsFromContext(ctx); ok {
		t.add(name, d)
	}
}

// Track starts a timer for the named phase and returns a stop function that
// records the elapsed time when called. Use as `defer Track(ctx, "parse")()` or
// keep the returned func to stop explicitly inside a loop.
func Track(ctx context.Context, name string) func() {
	start := time.Now()
	return func() {
		AddTiming(ctx, name, time.Since(start))
	}
}

func (t *Timings) add(name string, d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, seen := t.totals[name]; !seen {
		t.order = append(t.order, name)
	}
	t.totals[name] += d
}

// Header renders the accumulated phases as a Server-Timing value fragment
// (`find;dur=4.1, parse;dur=6.8`), durations in milliseconds to one decimal. It
// returns an empty string when no phases were recorded.
func (t *Timings) Header() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	parts := make([]string, 0, len(t.order))
	for _, name := range t.order {
		parts = append(parts, formatMetric(name, t.totals[name]))
	}
	return strings.Join(parts, ", ")
}

func formatMetric(name string, d time.Duration) string {
	return fmt.Sprintf("%s;dur=%.1f", name, float64(d)/float64(time.Millisecond))
}
