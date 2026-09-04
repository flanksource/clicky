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

	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

// scheduleEntry is one schedule's live state: the compiled spec, when it fires
// next, and the run it currently has in flight.
type scheduleEntry struct {
	schedule Schedule
	cron     cronSchedule
	next     time.Time

	// running is the in-flight run's group, nil when idle. queued records a
	// fire deferred under OverlapQueue, so at most one is ever pending — a
	// backlog of identical reports helps nobody.
	running *Group
	queued  *time.Time
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
		if err := s.Add(schedule); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("load schedules: %w", errsJoin(errs))
	}
	return nil
}

// Add registers or replaces one schedule. The next fire is computed from now,
// except that a schedule asking to catch up on a scheduled time missed while the
// process was down fires immediately instead.
func (s *Scheduler) Add(schedule Schedule) error {
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
	} else {
		entry.next = parsed.Next(now)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Replacing a schedule keeps the in-flight run: it belongs to the work, not
	// to the definition that started it.
	if existing, ok := s.entries[schedule.Name]; ok {
		entry.running = existing.running
	}
	s.entries[schedule.Name] = entry
	return nil
}

// Remove stops firing the named schedule. The in-flight run, if any, is left to
// finish — cancelling work because its schedule was deleted loses a result
// nobody asked to throw away.
func (s *Scheduler) Remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, name)
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
func (s *Scheduler) Start(ctx flanksourceContext.Context) {
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

// Stop ends the scheduler and waits for its loop to exit.
func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
	<-s.done
}

// RunDue fires every schedule whose time has come. It is the whole scheduling
// decision, exported so a test can drive it directly instead of waiting on a
// ticker.
func (s *Scheduler) RunDue(ctx flanksourceContext.Context) {
	now := s.now()

	s.mu.Lock()
	due := make([]*scheduleEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		if !entry.schedule.Enabled {
			continue
		}
		if entry.running == nil && entry.queued != nil && entry.next.After(now) {
			// A queued fire whose turn came because the previous run ended.
			due = append(due, entry)
			continue
		}
		if !entry.next.After(now) {
			due = append(due, entry)
		}
	}
	s.mu.Unlock()

	for _, entry := range due {
		s.fire(ctx, entry, now)
	}
}

// fire advances one schedule past now and starts, defers, or skips the run
// according to its overlap policy.
func (s *Scheduler) fire(ctx flanksourceContext.Context, entry *scheduleEntry, now time.Time) {
	s.mu.Lock()

	scheduledFor := entry.next
	if entry.queued != nil {
		scheduledFor = *entry.queued
		entry.queued = nil
	}
	if !entry.next.After(now) {
		entry.next = entry.cron.Next(now)
	}

	schedule := entry.schedule
	running := entry.running
	if running != nil && isRunning(running.Status()) {
		policy := schedule.overlapPolicy()
		switch policy {
		case OverlapSkip:
			s.mu.Unlock()
			s.record(ctx, schedule.Name, Fire{
				ScheduledFor: scheduledFor,
				At:           now,
				Outcome:      FireSkipped,
				RunID:        running.ID(),
				Reason:       "previous run still in progress",
			})
			return
		case OverlapQueue:
			queued := scheduledFor
			entry.queued = &queued
			s.mu.Unlock()
			s.record(ctx, schedule.Name, Fire{
				ScheduledFor: scheduledFor,
				At:           now,
				Outcome:      FireSkipped,
				RunID:        running.ID(),
				Reason:       "queued behind the previous run",
			})
			return
		case OverlapCancelPrevious:
			running.Cancel()
		}
	}
	s.mu.Unlock()

	outcome := FireStarted
	if entry.queued == nil && scheduledFor.Before(now.Truncate(time.Minute)) {
		outcome = FireCaughtUp
	}
	s.start(ctx, entry, schedule, scheduledFor, now, outcome)
}

// start launches the run for one fire and records the outcome.
func (s *Scheduler) start(
	ctx flanksourceContext.Context,
	entry *scheduleEntry,
	schedule Schedule,
	scheduledFor, now time.Time,
	outcome FireOutcome,
) {
	runner, ok := runnerFor(schedule.Kind)
	if !ok {
		s.record(ctx, schedule.Name, Fire{
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
	entry.schedule.LastRun = &now
	s.mu.Unlock()

	s.record(ctx, schedule.Name, Fire{
		ScheduledFor: scheduledFor, At: now, Outcome: outcome, RunID: group.ID(),
	})

	go func() {
		runCtx := ctx
		if schedule.Timeout > 0 {
			var cancel context.CancelFunc
			runCtx, cancel = ctx.WithTimeout(schedule.Timeout)
			defer cancel()
		}
		if err := runner(runCtx, schedule, group.Group); err != nil {
			logger.Warnf("schedule %s: %v", schedule.Name, err)
		}
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
