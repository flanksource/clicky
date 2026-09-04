package task

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// OverlapPolicy decides what happens when a schedule fires while its previous
// run is still going. There is no safe default for every workload — a cheap
// health probe wants to skip, an hourly report wants to queue — so it is a
// per-schedule choice rather than a scheduler-wide rule.
type OverlapPolicy string

const (
	// OverlapSkip drops the fire and records why. The default.
	OverlapSkip OverlapPolicy = "skip"
	// OverlapQueue runs the new fire once the previous one finishes. A schedule
	// that is persistently slower than its interval will back up.
	OverlapQueue OverlapPolicy = "queue"
	// OverlapCancelPrevious stops the in-flight run and starts the new one, for
	// schedules where only the latest result is worth anything.
	OverlapCancelPrevious OverlapPolicy = "cancel-previous"
)

// CatchUpPolicy decides what happens to scheduled times that passed while the
// process was down.
type CatchUpPolicy string

const (
	// CatchUpNone ignores missed times and resumes at the next one. The default.
	CatchUpNone CatchUpPolicy = "none"
	// CatchUpOnce runs once at startup if any scheduled time was missed,
	// regardless of how many were. Replaying every missed fire is never what a
	// reporting schedule wants after a weekend of downtime.
	CatchUpOnce CatchUpPolicy = "once"
)

// Schedule is a recurring run: when to fire, what to stamp on the runs it
// produces, and how to behave when firing collides with reality.
//
// The work itself is not here. A schedule names a Kind, and the host registers
// the function for that kind with RegisterRunner — the task package knows how to
// keep time, not what the work means.
type Schedule struct {
	Name string `json:"name"`

	// Kind selects the registered runner and is stamped on every run, so a
	// listing can filter to one schedule's kind of work.
	Kind   string            `json:"kind"`
	Labels map[string]string `json:"labels,omitempty"`
	Owner  string            `json:"owner,omitempty"`

	// Cron is a robfig/cron spec, including the descriptor forms ("@hourly",
	// "@every 5m"). Seconds are not accepted; the smallest unit is a minute.
	Cron string `json:"cron"`

	// Timezone is an IANA name. Empty means the process's local time, which is
	// what a bare "0 9 * * *" means to whoever typed it.
	Timezone string `json:"timezone,omitempty"`

	Enabled bool          `json:"enabled"`
	Timeout time.Duration `json:"timeout,omitempty"`

	Overlap OverlapPolicy `json:"overlap,omitempty"`
	CatchUp CatchUpPolicy `json:"catchUp,omitempty"`

	// LastRun and NextRun are maintained by the scheduler and persisted so a
	// restart can tell whether a scheduled time was missed.
	LastRun *time.Time `json:"lastRun,omitempty"`
	NextRun *time.Time `json:"nextRun,omitempty"`
}

// scheduleParser accepts the descriptor forms ("@hourly", "@every 1h") on top of
// standard five-field cron. Seconds are deliberately not enabled: a task manager
// that fires sub-minute is a ticker, and the caller wanted a ticker.
var scheduleParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// minimumInterval is the finest cadence a schedule may ask for. The cron field
// set stops at minutes, but "@every 5s" slips past that — and the scheduler
// wakes on a coarse tick, so it would fire late and irregularly rather than
// every five seconds. Refusing it where the schedule is configured beats
// silently under-delivering it forever.
const minimumInterval = time.Minute

// Validate reports whether the schedule can actually be run, naming the field at
// fault. A schedule that cannot fire is a configuration error worth refusing at
// the point it is saved, not a run that silently never happens.
func (s Schedule) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("schedule name is required")
	}
	if strings.TrimSpace(s.Kind) == "" {
		return fmt.Errorf("schedule %q: kind is required", s.Name)
	}
	if _, err := s.Location(); err != nil {
		return err
	}
	if _, err := s.Parse(); err != nil {
		return err
	}
	switch s.Overlap {
	case "", OverlapSkip, OverlapQueue, OverlapCancelPrevious:
	default:
		return fmt.Errorf("schedule %q: unknown overlap policy %q", s.Name, s.Overlap)
	}
	switch s.CatchUp {
	case "", CatchUpNone, CatchUpOnce:
	default:
		return fmt.Errorf("schedule %q: unknown catch-up policy %q", s.Name, s.CatchUp)
	}
	if s.Timeout < 0 {
		return fmt.Errorf("schedule %q: timeout must not be negative", s.Name)
	}
	return nil
}

// Location resolves the schedule's timezone.
func (s Schedule) Location() (*time.Location, error) {
	if strings.TrimSpace(s.Timezone) == "" {
		return time.Local, nil
	}
	location, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return nil, fmt.Errorf("schedule %q: unknown timezone %q: %w", s.Name, s.Timezone, err)
	}
	return location, nil
}

// Parse compiles the cron spec in the schedule's own timezone.
func (s Schedule) Parse() (cron.Schedule, error) {
	if strings.TrimSpace(s.Cron) == "" {
		return nil, fmt.Errorf("schedule %q: cron is required", s.Name)
	}
	location, err := s.Location()
	if err != nil {
		return nil, err
	}
	spec := strings.TrimSpace(s.Cron)
	// A TZ= prefix inside the spec silently wins over the field, so a schedule
	// carrying both is one whose two answers disagree and whose owner cannot
	// see which one won. Refuse it instead of picking for them.
	inlineZone := strings.HasPrefix(spec, "TZ=") || strings.HasPrefix(spec, "CRON_TZ=")
	if inlineZone && strings.TrimSpace(s.Timezone) != "" {
		return nil, fmt.Errorf(
			"schedule %q: cron %q already sets a timezone and timezone %q also does; set one of them",
			s.Name, s.Cron, s.Timezone)
	}
	if !inlineZone && location != time.Local {
		spec = "CRON_TZ=" + location.String() + " " + spec
	}
	parsed, err := scheduleParser.Parse(spec)
	if err != nil {
		return nil, fmt.Errorf("schedule %q: invalid cron %q: %w", s.Name, s.Cron, err)
	}
	if every, ok := parsed.(cron.ConstantDelaySchedule); ok && every.Delay < minimumInterval {
		return nil, fmt.Errorf(
			"schedule %q: invalid cron %q: fires every %s, and the scheduler cannot honour anything under %s",
			s.Name, s.Cron, every.Delay, minimumInterval)
	}
	return parsed, nil
}

// overlapPolicy returns the effective policy, resolving the empty default.
func (s Schedule) overlapPolicy() OverlapPolicy {
	if s.Overlap == "" {
		return OverlapSkip
	}
	return s.Overlap
}

// catchUpPolicy returns the effective policy, resolving the empty default.
func (s Schedule) catchUpPolicy() CatchUpPolicy {
	if s.CatchUp == "" {
		return CatchUpNone
	}
	return s.CatchUp
}

// missedFire reports the scheduled time that passed unrun while the process was
// down, if the schedule asks to catch up on one. It is derived from NextRun as
// persisted before the shutdown: that instant is precisely the promise the
// previous process made and did not keep.
func (s Schedule) missedFire(now time.Time) (time.Time, bool) {
	if s.catchUpPolicy() != CatchUpOnce || s.NextRun == nil {
		return time.Time{}, false
	}
	if s.NextRun.After(now) {
		return time.Time{}, false
	}
	return *s.NextRun, true
}
