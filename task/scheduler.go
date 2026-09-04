package task

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/flanksource/commons/logger"
)

// Runner is the work behind a schedule. The group is already registered and
// stamped with the schedule's kind and labels; the runner adds tasks to it.
type Runner func(ctx flanksourceContext.Context, schedule Schedule, group *Group) error

var (
	runnersMu sync.RWMutex
	runners   = map[string]Runner{}
)

// RegisterRunner registers the work for one schedule kind. Registering a kind
// twice is a programming error and panics, matching the entity and provider
// registries elsewhere: two runners for one kind means one of them never runs.
func RegisterRunner(kind string, runner Runner) {
	if kind == "" || runner == nil {
		panic("task: RegisterRunner requires a kind and a runner")
	}
	runnersMu.Lock()
	defer runnersMu.Unlock()
	if _, exists := runners[kind]; exists {
		panic(fmt.Sprintf("task: runner for kind %q already registered", kind))
	}
	runners[kind] = runner
}

func runnerFor(kind string) (Runner, bool) {
	runnersMu.RLock()
	defer runnersMu.RUnlock()
	runner, ok := runners[kind]
	return runner, ok
}

// schedulerTick is how often the scheduler wakes to compare wall time against
// each schedule's next fire. Schedules resolve to at most one fire per minute,
// so a coarser tick than this buys nothing and a finer one only burns wakeups.
const schedulerTick = 10 * time.Second

// Scheduler fires schedules and turns each fire into a tracked run. It owns no
// work of its own: the run is a Group, so it is visible, filterable and
// controllable through exactly the same API as anything else the manager runs.
type Scheduler struct {
	store Store
	now   func() time.Time
	tick  time.Duration

	mu      sync.Mutex
	entries map[string]*scheduleEntry

	// started and stopped make the lifecycle single-shot: exactly one loop is
	// ever launched, and Stop is answerable before one exists.
	started bool
	stopped bool

	stop chan struct{}
	done chan struct{}
}

// scheduleEntry is one schedule's live state: the compiled spec, when it fires
// next, and the run it currently has in flight.
type scheduleEntry struct {
	schedule Schedule
	cron     cronSchedule
	next     time.Time

	// running is the in-flight run's group, nil when idle. launching reserves
	// that same slot for the window between the firing decision and the group
	// existing, so two concurrent RunDue calls cannot both conclude the
	// schedule is idle. queued records a fire deferred under OverlapQueue, so
	// at most one is ever pending — a backlog of identical reports helps
	// nobody.
	running   *Group
	launching bool
	queued    *time.Time

	// catchUp marks the queued instant as one missed while the process was
	// down, which is a different fact about the run than "it waited its turn"
	// and is reported as such.
	catchUp bool
}

// busy reports whether the schedule already has a run in flight or on its way
// there. Callers hold the scheduler's lock.
func (e *scheduleEntry) busy() bool {
	if e.launching {
		return true
	}
	return e.running != nil && isRunning(e.running.Status())
}

// cronSchedule is the slice of robfig/cron the scheduler needs, named so the
// dependency stays at one method and a fake clock can drive tests.
type cronSchedule interface {
	Next(time.Time) time.Time
}

// SchedulerOptions configure a Scheduler. Now and Tick exist for tests; both
// have working defaults.
type SchedulerOptions struct {
	Store Store
	Now   func() time.Time
	Tick  time.Duration
}

// NewScheduler creates a scheduler. A nil store is allowed — schedules are then
// in-memory only and do not survive a restart, which is what a CLI wants.
func NewScheduler(options SchedulerOptions) *Scheduler {
	s := &Scheduler{
		store:   options.Store,
		now:     options.Now,
		tick:    options.Tick,
		entries: map[string]*scheduleEntry{},
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.tick <= 0 {
		s.tick = schedulerTick
	}
	return s
}

// Load reads the persisted schedules into the scheduler, replacing whatever it
// held. Schedules that no longer validate are reported rather than dropped
// silently: a schedule that stopped being runnable is the operator's problem to
// see, not the scheduler's to hide.
func (s *Scheduler) Load(ctx context.Context) error {
	if s.store == nil {
		return nil
	}
	schedules, err := s.store.ListSchedules(ctx)
	if err != nil {
		return fmt.Errorf("list schedules: %w", err)
	}
	var errs []error
	for _, schedule := range schedules {
		if err := s.Add(ctx, schedule); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("load schedules: %w", errsJoin(errs))
	}
	return nil
}

// Add registers or replaces one schedule and persists the definition. The next
// fire is computed from now, except that a schedule asking to catch up on a
// scheduled time missed while the process was down fires immediately instead.
func (s *Scheduler) Add(ctx context.Context, schedule Schedule) error {
	if err := schedule.Validate(); err != nil {
		return err
	}
	if _, ok := runnerFor(schedule.Kind); !ok {
		return fmt.Errorf("schedule %q: no runner registered for kind %q", schedule.Name, schedule.Kind)
	}
	parsed, err := schedule.Parse()
	if err != nil {
		return err
	}

	now := s.now()
	entry := &scheduleEntry{schedule: schedule, cron: parsed}
	if missed, ok := schedule.missedFire(now); ok {
		// Fire at the next tick, but keep the instant the run is *for* so the
		// history says which scheduled time this made good on.
		entry.next = now
		entry.queued = &missed
		entry.catchUp = true
	} else {
		entry.next = parsed.Next(now)
	}

	s.mu.Lock()
	// Replacing a schedule updates the entry in place rather than swapping in a
	// new one: the in-flight run and its launch reservation belong to the work,
	// not to the definition that started it, and a stable pointer per name is
	// what lets fire tell "replaced" apart from "removed".
	if existing, ok := s.entries[schedule.Name]; ok {
		existing.schedule = schedule
		existing.cron = entry.cron
		existing.next = entry.next
		existing.queued = entry.queued
		existing.catchUp = entry.catchUp
	} else {
		s.entries[schedule.Name] = entry
	}
	s.mu.Unlock()

	// Persisted outside the lock — the store may block on IO — and only once
	// the in-memory mutation has committed, so what is saved is what is live.
	if err := s.saveSchedule(ctx, schedule.Name); err != nil {
		return fmt.Errorf("schedule %q: save: %w", schedule.Name, err)
	}
	return nil
}

// Remove stops firing the named schedule and deletes its stored definition, so
// a restart does not resurrect it. The in-flight run, if any, is left to
// finish — cancelling work because its schedule was deleted loses a result
// nobody asked to throw away.
func (s *Scheduler) Remove(ctx context.Context, name string) error {
	s.mu.Lock()
	delete(s.entries, name)
	s.mu.Unlock()

	if s.store == nil {
		return nil
	}
	// Deleted even when the entry was not in memory: a definition that failed
	// to load is exactly the one an operator is trying to get rid of.
	if err := s.store.DeleteSchedule(ctx, name); err != nil {
		return fmt.Errorf("schedule %q: delete: %w", name, err)
	}
	return nil
}

// saveSchedule persists one definition together with the timing the scheduler
// has just recomputed. The snapshot is taken under the lock and the store call
// made outside it: the store may block on IO, and no firing decision may wait
// behind that.
func (s *Scheduler) saveSchedule(ctx context.Context, name string) error {
	if s.store == nil {
		return nil
	}
	s.mu.Lock()
	entry, ok := s.entries[name]
	var schedule Schedule
	if ok {
		schedule = entry.schedule
		next := entry.next
		schedule.NextRun = &next
	}
	s.mu.Unlock()
	if !ok {
		return nil
	}
	return s.store.SaveSchedule(ctx, schedule)
}

// persistTiming saves the schedule after a firing decision moved its NextRun or
// LastRun. A failure here loses catch-up accuracy across a restart, not the run
// itself, so it is reported and the fire continues.
func (s *Scheduler) persistTiming(ctx context.Context, name string) {
	if err := s.saveSchedule(ctx, name); err != nil {
		logger.Warnf("schedule %s: save: %v", name, err)
	}
}

// Schedules returns the current schedules, newest fire first.
func (s *Scheduler) Schedules() []Schedule {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Schedule, 0, len(s.entries))
	for _, entry := range s.entries {
		schedule := entry.schedule
		next := entry.next
		schedule.NextRun = &next
		out = append(out, schedule)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Start runs the scheduler until ctx is done or Stop is called. It registers
// itself as a managed background run so it is visible in the task UI without
// occupying a worker or blocking Wait.
//
// Starting an already-started or already-stopped scheduler is a no-op: there is
// exactly one loop, and it is the one thing allowed to close done.
func (s *Scheduler) Start(ctx flanksourceContext.Context) {
	s.mu.Lock()
	if s.started || s.stopped {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	run := StartManagedRun("scheduler", WithKind("scheduler"))
	run.SetBackground(true)

	go func() {
		defer close(s.done)
		defer run.Finish(StatusSuccess, nil)

		ticker := time.NewTicker(s.tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stop:
				return
			case <-ticker.C:
				s.RunDue(ctx)
			}
		}
	}()
}

// Stop ends the scheduler and waits for its loop to exit. Safe before Start and
// safe to repeat: a scheduler that never started has no loop to wait for, and
// one already stopped has nothing left to close.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	started := s.started
	if !s.stopped {
		s.stopped = true
		close(s.stop)
	}
	s.mu.Unlock()

	if started {
		<-s.done
	}
}

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

	s.start(ctx, name, entry, schedule, scheduledFor, now, outcome)
}

// start launches the run for one fire and records the outcome.
func (s *Scheduler) start(
	ctx flanksourceContext.Context,
	name string,
	entry *scheduleEntry,
	schedule Schedule,
	scheduledFor, now time.Time,
	outcome FireOutcome,
) {
	runner, ok := runnerFor(schedule.Kind)
	if !ok {
		s.mu.Lock()
		entry.launching = false
		s.mu.Unlock()
		s.record(ctx, name, Fire{
			ScheduledFor: scheduledFor, At: now, Outcome: FireFailed,
			Error: fmt.Sprintf("no runner registered for kind %q", schedule.Kind),
		})
		return
	}

	labels := map[string]string{"schedule": schedule.Name}
	for k, v := range schedule.Labels {
		labels[k] = v
	}

	group := StartGroup[any](schedule.Name,
		WithKind(schedule.Kind),
		WithLabels(labels),
		WithOwner(schedule.Owner),
	)

	s.mu.Lock()
	entry.running = group.Group
	entry.launching = false
	entry.schedule.LastRun = &now
	s.mu.Unlock()

	// The runner is a task in its own group rather than a bare goroutine, so a
	// runner that fails before adding any child leaves a failed run instead of
	// a group stuck pending and an error visible only in the log. The children
	// it adds join the same group and are waited for below.
	var options []Option
	if schedule.Timeout > 0 {
		options = append(options, WithTimeout(schedule.Timeout))
	}
	group.Add(schedule.Kind, func(runCtx flanksourceContext.Context, _ *Task) (any, error) {
		return nil, runner(runCtx, schedule, group.Group)
	}, options...)

	s.persistTiming(ctx, name)
	s.record(ctx, name, Fire{
		ScheduledFor: scheduledFor, At: now, Outcome: outcome, RunID: group.ID(),
	})

	go func() {
		group.WaitFor()

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
	}()
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

func errsJoin(errs []error) error {
	if len(errs) == 1 {
		return errs[0]
	}
	message := ""
	for i, err := range errs {
		if i > 0 {
			message += "; "
		}
		message += err.Error()
	}
	return fmt.Errorf("%s", message)
}
