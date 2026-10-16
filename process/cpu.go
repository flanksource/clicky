package process

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// CPUSample is one observation of a process: the listing entry and the wall
// clock time the listing was taken.
type CPUSample struct {
	Process Process
	At      time.Time
}

// IntervalCPUPercent returns the CPU% a process used between two samples:
// CPU time consumed divided by wall time elapsed, ×100, per core like top
// (a process saturating two cores reads 200).
//
// The result is nil, meaning unknown, when there is no previous sample or the
// pid was reused by a different process (start times differ): a rate needs two
// observations of the same process, and a fake 0 would read as idle. Samples
// that cannot be compared (different pids, missing start time, a clock that
// did not advance, CPU time going backwards) are errors.
func IntervalCPUPercent(previous *CPUSample, current CPUSample) (*float64, error) {
	if previous == nil {
		return nil, nil
	}
	if previous.Process.PID != current.Process.PID {
		return nil, fmt.Errorf("interval cpu: comparing pid %d with pid %d", previous.Process.PID, current.Process.PID)
	}
	if previous.Process.StartedAt == nil || current.Process.StartedAt == nil {
		return nil, fmt.Errorf("interval cpu: pid %d has no start time to tell pid reuse apart", current.Process.PID)
	}
	if !previous.Process.StartedAt.Equal(*current.Process.StartedAt) {
		return nil, nil
	}
	wall := current.At.Sub(previous.At)
	if wall <= 0 {
		return nil, fmt.Errorf("interval cpu: pid %d clock did not advance (%s -> %s)", current.Process.PID, previous.At, current.At)
	}
	used := current.Process.CPUTime - previous.Process.CPUTime
	if used < 0 {
		return nil, fmt.Errorf("interval cpu: pid %d cpu time went backwards (%s -> %s)", current.Process.PID, previous.Process.CPUTime, current.Process.CPUTime)
	}
	percent := used.Seconds() / wall.Seconds() * 100
	return &percent, nil
}

// parseCPUTime parses a ps time column, [[dd-]hh:]mm:ss[.ff]: linux prints
// [dd-]hh:mm:ss, darwin prints mm:ss.ff with minutes unbounded.
func parseCPUTime(value string) (time.Duration, error) {
	var total time.Duration
	clock := value
	if days, rest, found := strings.Cut(value, "-"); found {
		count, err := strconv.ParseUint(days, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("ps cpu time %q: days: %w", value, err)
		}
		total, clock = time.Duration(count)*24*time.Hour, rest
	}
	parts := strings.Split(clock, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("ps cpu time %q: want [[dd-]hh:]mm:ss[.ff]", value)
	}
	seconds, err := strconv.ParseFloat(parts[len(parts)-1], 64)
	if err != nil || seconds < 0 {
		return 0, fmt.Errorf("ps cpu time %q: invalid seconds", value)
	}
	total += time.Duration(math.Round(seconds * float64(time.Second)))
	units := []time.Duration{time.Minute, time.Hour}
	for i, part := range parts[:len(parts)-1] {
		count, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("ps cpu time %q: %w", value, err)
		}
		total += time.Duration(count) * units[len(parts)-2-i]
	}
	return total, nil
}
