package clicky

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	rpchttp "github.com/flanksource/clicky/rpc/http"
	"github.com/spf13/pflag"
)

// TimingsStderr is the --timings value of a bare flag: print the report as a
// Server-Timing line on stderr instead of writing a JSON file.
const TimingsStderr = "-"

// TimingsOptions is the CLI counterpart of rpchttp.TimingMiddleware: the same
// request-scoped accumulator every AddTiming call (git, SQL, cache, HTTP)
// feeds, installed on the command context and reported when the command ends.
type TimingsOptions struct {
	// Timings is "" (off), TimingsStderr, or the path of a TimingsReport.
	Timings string

	run *timingsRun
}

// timingsRun is the state of one started report, behind a pointer so the
// options stay copyable like the rest of AllFlags.
type timingsRun struct {
	mu       sync.Mutex
	timings  *rpchttp.Timings
	started  time.Time
	reported bool
}

// TimingsReport is the --timings=<path> JSON document.
type TimingsReport struct {
	WallMs  float64         `json:"wallMs"`
	Process ProcessUsage    `json:"process"`
	Metrics []TimingsMetric `json:"metrics"`
}

// TimingsMetric is one accumulated timing metric.
type TimingsMetric struct {
	Name       string           `json:"name"`
	DurationMs float64          `json:"durationMs"`
	Counters   map[string]int64 `json:"counters,omitempty"`
}

// ProcessUsage is getrusage for the process and for its waited-for children
// (subprocesses, cumulatively; MaxRSSBytes is the largest one).
type ProcessUsage struct {
	Self     ResourceUsage `json:"self"`
	Children ResourceUsage `json:"children"`
}

// ResourceUsage is one getrusage reading.
type ResourceUsage struct {
	UserUs      int64 `json:"userUs"`
	SysUs       int64 `json:"sysUs"`
	MaxRSSBytes int64 `json:"maxRssBytes"`
	InBlocks    int64 `json:"inBlocks"`
	OutBlocks   int64 `json:"outBlocks"`
}

// BindFlags adds --timings to flags.
func (o *TimingsOptions) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.Timings, "timings", "",
		"Report the command's timings (wall, process and subprocess CPU, peak RSS, block IO, and git/SQL/cache counters): "+
			"a bare --timings prints a Server-Timing line to stderr, --timings=<path> writes JSON")
	flags.Lookup("timings").NoOptDefVal = TimingsStderr
}

// StartTimings returns ctx carrying the accumulator when --timings is set.
// Run the command under it and call StopTimings once it returns.
func (o *TimingsOptions) StartTimings(ctx context.Context) context.Context {
	if o.Timings == "" || o.run != nil {
		return ctx
	}
	o.run = &timingsRun{started: time.Now()}
	ctx, o.run.timings = rpchttp.WithTimings(ctx)
	return ctx
}

// StopTimings reports what the command accumulated. It reports once, so both
// a signal path and the normal exit may call it.
func (o *TimingsOptions) StopTimings() error {
	run := o.run
	if run == nil {
		return nil
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.reported {
		return nil
	}
	run.reported = true
	report := run.report()
	if o.Timings == TimingsStderr {
		_, err := fmt.Fprintf(os.Stderr, "Server-Timing: total;dur=%.1f;desc=\"cpu_us=%d child_cpu_us=%d max_rss_bytes=%d child_max_rss_bytes=%d\", %s\n",
			report.WallMs, report.Process.Self.UserUs+report.Process.Self.SysUs, report.Process.Children.UserUs+report.Process.Children.SysUs,
			report.Process.Self.MaxRSSBytes, report.Process.Children.MaxRSSBytes, run.timings.Header())
		return err
	}
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(o.Timings, append(body, '\n'), 0o644); err != nil {
		return fmt.Errorf("write timings report: %w", err)
	}
	return nil
}

func (run *timingsRun) report() TimingsReport {
	report := TimingsReport{WallMs: milliseconds(time.Since(run.started)), Process: processUsage(), Metrics: []TimingsMetric{}}
	for _, metric := range run.timings.Metrics() {
		entry := TimingsMetric{Name: metric.Name, DurationMs: milliseconds(metric.Duration)}
		for _, counter := range metric.Counters {
			if entry.Counters == nil {
				entry.Counters = map[string]int64{}
			}
			entry.Counters[counter.Name] = counter.Value
		}
		report.Metrics = append(report.Metrics, entry)
	}
	return report
}

func milliseconds(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
