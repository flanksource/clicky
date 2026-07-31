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
	"strconv"
	"strings"
	"sync"
	"time"
)

// timingsKey keys the request-scoped *Timings accumulator into a context.
type timingsKey struct{}

// TimingCounter is a named integer aggregated into a metric's description.
type TimingCounter struct {
	Name  string
	Value int64
}

// TimingMetric is one contribution to a request-scoped server timing metric.
type TimingMetric struct {
	Name     string
	Duration time.Duration
	Counters []TimingCounter
}

type timingTotal struct {
	duration     time.Duration
	counterOrder []string
	counters     map[string]int64
}

// Timings is a request-scoped, insertion-ordered accumulator of named metrics.
type Timings struct {
	mu      sync.Mutex
	order   []string
	metrics map[string]*timingTotal
}

// WithTimings returns a context carrying a fresh accumulator and the accumulator
// itself, so a middleware can read it back after the handler returns.
func WithTimings(ctx context.Context) (context.Context, *Timings) {
	t := &Timings{metrics: make(map[string]*timingTotal)}
	return context.WithValue(ctx, timingsKey{}, t), t
}

// TimingsFromContext returns the accumulator installed by WithTimings, if any.
func TimingsFromContext(ctx context.Context) (*Timings, bool) {
	t, ok := ctx.Value(timingsKey{}).(*Timings)
	return t, ok
}

// AddTiming aggregates a metric into the current request. It is a no-op when no
// accumulator is present, such as on the CLI path.
func AddTiming(ctx context.Context, metric TimingMetric) {
	if t, ok := TimingsFromContext(ctx); ok {
		t.add(metric)
	}
}

// Track starts a timer for the named phase and returns a stop function that
// records the elapsed time when called. Use as `defer Track(ctx, "parse")()` or
// keep the returned func to stop explicitly inside a loop.
func Track(ctx context.Context, name string) func() {
	start := time.Now()
	return func() {
		AddTiming(ctx, TimingMetric{Name: name, Duration: time.Since(start)})
	}
}

func (t *Timings) add(metric TimingMetric) {
	t.mu.Lock()
	defer t.mu.Unlock()
	total, seen := t.metrics[metric.Name]
	if !seen {
		total = &timingTotal{counters: make(map[string]int64)}
		t.metrics[metric.Name] = total
		t.order = append(t.order, metric.Name)
	}
	total.duration += metric.Duration
	for _, counter := range metric.Counters {
		if _, seen := total.counters[counter.Name]; !seen {
			total.counterOrder = append(total.counterOrder, counter.Name)
		}
		total.counters[counter.Name] += counter.Value
	}
}

// Header renders the accumulated phases as a Server-Timing value fragment
// (`find;dur=4.1, parse;dur=6.8`), durations in milliseconds to one decimal. It
// returns an empty string when no phases were recorded.
func (t *Timings) Header() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	parts := make([]string, 0, len(t.order))
	for _, name := range t.order {
		total := t.metrics[name]
		parts = append(parts, formatMetric(name, total.duration, total.counterDescription()))
	}
	return strings.Join(parts, ", ")
}

func (t *timingTotal) counterDescription() string {
	if len(t.counterOrder) == 0 {
		return ""
	}
	parts := make([]string, 0, len(t.counterOrder))
	for _, name := range t.counterOrder {
		parts = append(parts, name+"="+strconv.FormatInt(t.counters[name], 10))
	}
	return strings.Join(parts, " ")
}

func formatMetric(name string, d time.Duration, description string) string {
	metric := fmt.Sprintf("%s;dur=%.1f", name, float64(d)/float64(time.Millisecond))
	if description == "" {
		return metric
	}
	description = strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(description)
	return metric + `;desc="` + description + `"`
}
