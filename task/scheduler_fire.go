package task

import (
	"context"
	"fmt"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/flanksource/commons/logger"
)

// RunDue fires every schedule whose time has come. It is the whole scheduling
// decision, exported so a test can drive it directly instead of waiting on a
// ticker.
func (s *Scheduler) RunDue(ctx flanksourceContext.Context) {
	now := s.now()

	s.mu.Lock()
	due := make([]string, 0, len(s.entries))
	for name, entry := range s.entries {
		if !entry.schedule.Enabled {
			continue
		}
		if !entry.busy() && entry.queued != nil && entry.next.After(now) {
			// A queued fire whose turn came because the previous run ended.
			due = append(due, name)
			continue
		}
		if !entry.next.After(now) {
			due = append(due, name)
		}
	}
	s.mu.Unlock()

	for _, name := range due {
		s.fire(ctx, name, now)
	}
}

// fire advances one schedule past now and starts, defers, or skips the run
// according to its overlap policy.
//
// The entry is resolved again here rather than carried over from RunDue's scan:
// a schedule removed in between must not run, and the decision and the launch
// reservation that follows it have to be one atomic step, or two ticks racing
// each other both find the schedule idle.
func (s *Scheduler) fire(ctx flanksourceContext.Context, name string, now time.Time) {
	s.mu.Lock()
	entry, ok := s.entries[name]
	if !ok {
		s.mu.Unlock()
		return
	}

	// due is the instant this tick is acting on. Anything already queued stays
	// queued until it can actually start, so a fire that is itself deferred is
	// recorded as the fire it was rather than replaying an older one.
	due := entry.next
	if !entry.next.After(now) {
		entry.next = entry.cron.Next(now)
	}

	schedule := entry.schedule
	if entry.busy() {
		running := entry.running
		runID := ""
		if running != nil {
			runID = running.ID()
		}
		switch schedule.overlapPolicy() {
		case OverlapSkip:
			s.mu.Unlock()
			s.persistTiming(ctx, name)
			s.record(ctx, name, Fire{
				ScheduledFor: due,
				At:           now,
				Outcome:      FireSkipped,
				RunID:        runID,
				Reason:       "previous run still in progress",
			})
			return
		case OverlapQueue:
			// The oldest unrun instant is the one still owed, so it keeps the
			// slot; this tick is recorded as skipped in its own right.
			if entry.queued == nil {
				queued := due
				entry.queued = &queued
			}
			s.mu.Unlock()
			s.persistTiming(ctx, name)
			s.record(ctx, name, Fire{
				ScheduledFor: due,
				At:           now,
				Outcome:      FireSkipped,
				RunID:        runID,
				Reason:       "queued behind the previous run",
			})
			return
		case OverlapCancelPrevious:
			if running != nil {
				running.Cancel()
			}
		}
	}

	scheduledFor := due
	outcome := FireStarted
	if entry.queued != nil {
		scheduledFor = *entry.queued
		entry.queued = nil
		if entry.catchUp {
			outcome = FireCaughtUp
			entry.catchUp = false
		}
	}
	entry.launching = true
	s.mu.Unlock()

	_, _ = s.start(ctx, name, entry, schedule, scheduledFor, now, outcome, nil)
}

// start launches the run for one fire and records the outcome.
func (s *Scheduler) start(
	ctx flanksourceContext.Context,
	name string,
	entry *scheduleEntry,
	schedule Schedule,
	scheduledFor, now time.Time,
	outcome FireOutcome,
	extraLabels map[string]string,
) (*Group, error) {
	runner, ok := s.runnerFor(schedule.Kind)
	if !ok {
		if entry != nil {
			s.mu.Lock()
			entry.launching = false
			s.mu.Unlock()
		}
		err := fmt.Errorf("no runner registered for kind %q", schedule.Kind)
		s.record(ctx, name, Fire{
			ScheduledFor: scheduledFor, At: now, Outcome: FireFailed,
			Error: err.Error(),
		})
		return nil, err
	}

	title := schedule.Title
	if title == "" {
		title = schedule.Name
	}
	group := StartGroup[any](title, runGroupOptions(schedule, extraLabels)...)

	if entry != nil {
		s.mu.Lock()
		entry.running = group.Group
		entry.launching = false
		entry.schedule.LastRun = &now
		s.mu.Unlock()
	}

	// The runner is a task in its own group rather than a bare goroutine, so a
	// runner that fails before adding any child leaves a failed run instead of
	// a group stuck pending and an error visible only in the log. The children
	// it adds join the same group and are waited for below.
	options := []Option{WithContext(context.WithoutCancel(ctx))}
	if schedule.Timeout > 0 {
		options = append(options, WithTimeout(schedule.Timeout))
	}
	group.Add(schedule.Kind, func(runCtx flanksourceContext.Context, _ *Task) (any, error) {
		return nil, runner(runCtx, schedule, group.Group)
	}, options...)

	if name != "" {
		s.persistTiming(ctx, name)
		s.record(ctx, name, Fire{
			ScheduledFor: scheduledFor, At: now, Outcome: outcome, RunID: group.ID(),
		})
	}

	go func() {
		group.WaitFor()

		if entry != nil {
			s.mu.Lock()
			if entry.running == group.Group {
				entry.running = nil
			}
			// A queued fire runs as soon as the slot frees, rather than waiting for
			// the next scheduled time it already missed.
			if entry.queued != nil {
				entry.next = s.now()
			}
			s.mu.Unlock()
		}
	}()
	return group.Group, nil
}

// runGroupOptions stamps the run with the schedule's identity and bounds how
// many of its tasks — the runner included — run at once.
func runGroupOptions(schedule Schedule, extraLabels map[string]string) []TaskGroupOption {
	labels := map[string]string{"schedule": schedule.Name}
	for k, v := range schedule.Labels {
		labels[k] = v
	}
	for k, v := range extraLabels {
		labels[k] = v
	}
	options := []TaskGroupOption{WithKind(schedule.Kind), WithLabels(labels), WithOwner(schedule.Owner)}
	if schedule.Concurrency > 0 {
		options = append(options, WithConcurrency(schedule.Concurrency))
	}
	return options
}

func (s *Scheduler) record(ctx context.Context, name string, fire Fire) {
	if s.store == nil {
		return
	}
	if err := s.store.RecordFire(ctx, name, fire); err != nil {
		logger.Warnf("schedule %s: record fire: %v", name, err)
	}
}

func isRunning(status Status) bool {
	return status == StatusRunning || status == StatusPending
}
