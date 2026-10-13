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

// schedulerTick is how often the scheduler wakes to compare wall time against
// each schedule's next fire. Schedules resolve to at most one fire per minute,
// so a coarser tick than this buys nothing and a finer one only burns wakeups.
const schedulerTick = 10 * time.Second

// Scheduler fires schedules and turns each fire into a tracked run. It owns no
// work of its own: the run is a Group, so it is visible, filterable and
// controllable through exactly the same API as anything else the manager runs.
type Scheduler struct {
	store ScheduleStore
	now   func() time.Time
	tick  time.Duration

	runnersMu sync.RWMutex
	runners   map[string]Runner

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
	Store ScheduleStore
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
		runners: map[string]Runner{},
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.tick <= 0 {
		s.tick = schedulerTick
	}
	return s
}

// RegisterRunner installs work owned by this scheduler instance. Instance
// runners keep independently configured services from sharing process-global
// execution state while the package-level registry remains available to CLI
// programs that deliberately use it.
func (s *Scheduler) RegisterRunner(kind string, runner Runner) {
	if kind == "" || runner == nil {
		panic("task: RegisterRunner requires a kind and a runner")
	}
	s.runnersMu.Lock()
	defer s.runnersMu.Unlock()
	if _, exists := s.runners[kind]; exists {
		panic(fmt.Sprintf("task: runner for kind %q already registered", kind))
	}
	s.runners[kind] = runner
}

func (s *Scheduler) runnerFor(kind string) (Runner, bool) {
	s.runnersMu.RLock()
	runner, ok := s.runners[kind]
	s.runnersMu.RUnlock()
	if ok {
		return runner, true
	}
	return runnerFor(kind)
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
//
// The definition is stored before the entry is installed, so an error means the
// schedule is not registered and will not fire: a caller that saw Add fail never
// has to wonder whether it left a live schedule behind. That also keeps Load's
// error aggregate honest — a schedule reported as failed to load is one that is
// genuinely absent from the scheduler.
func (s *Scheduler) Add(ctx context.Context, schedule Schedule) error {
	if err := schedule.Validate(); err != nil {
		return err
	}
	if _, ok := s.runnerFor(schedule.Kind); !ok {
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

	if s.store != nil {
		stored := schedule
		next := entry.next
		stored.NextRun = &next
		if err := s.store.SaveSchedule(ctx, stored); err != nil {
			return fmt.Errorf("schedule %q: save: %w", schedule.Name, err)
		}
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
	return nil
}

// Delete removes the stored definition before unregistering it, so a failed
// delete cannot leave a schedule live only until the next restart. The in-flight
// run, if any, is left to finish.
func (s *Scheduler) Delete(ctx context.Context, name string) error {
	if s.store != nil {
		if err := s.store.DeleteSchedule(ctx, name); err != nil {
			return fmt.Errorf("schedule %q: delete: %w", name, err)
		}
	}
	s.Unregister(name)
	return nil
}

// Unregister stops future fires without deleting the stored definition. It is
// useful when another service owns the definition row but delegates timing and
// execution to Scheduler.
func (s *Scheduler) Unregister(name string) {
	s.mu.Lock()
	delete(s.entries, name)
	s.mu.Unlock()
}

// SetEnabled pauses or resumes a schedule without replacing its definition.
// Resuming computes the next fire from now; time spent deliberately paused is
// not downtime and therefore never becomes catch-up work.
func (s *Scheduler) SetEnabled(ctx context.Context, name string, enabled bool) (Schedule, error) {
	s.mu.Lock()
	entry, ok := s.entries[name]
	if !ok {
		s.mu.Unlock()
		return Schedule{}, fmt.Errorf("schedule %q: not registered", name)
	}
	schedule := entry.schedule
	next := entry.next
	if enabled && !schedule.Enabled {
		next = entry.cron.Next(s.now())
	}
	schedule.Enabled = enabled
	schedule.NextRun = &next
	s.mu.Unlock()

	if s.store != nil {
		if err := s.store.SaveSchedule(ctx, schedule); err != nil {
			return Schedule{}, fmt.Errorf("schedule %q: save: %w", name, err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.entries[name]
	if !ok || current != entry {
		return Schedule{}, fmt.Errorf("schedule %q: changed while updating", name)
	}
	current.schedule.Enabled = enabled
	current.next = next
	current.queued = nil
	current.catchUp = false
	return schedule, nil
}

// Trigger starts one registered schedule immediately, including a disabled one.
// A manual run uses the same registered runner, timeout, labels and tracked task
// group as a cron fire, but does not advance the next cron instant.
func (s *Scheduler) Trigger(ctx flanksourceContext.Context, name string) (*Group, error) {
	now := s.now()
	s.mu.Lock()
	entry, ok := s.entries[name]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("schedule %q: not registered", name)
	}
	if entry.busy() {
		running := entry.running
		if entry.schedule.overlapPolicy() != OverlapCancelPrevious {
			s.mu.Unlock()
			return nil, fmt.Errorf("schedule %q: previous run still in progress", name)
		}
		if running != nil {
			running.Cancel()
		}
	}
	entry.launching = true
	schedule := entry.schedule
	s.mu.Unlock()

	return s.start(ctx, name, entry, schedule, now, now, FireManual, map[string]string{"trigger": "manual"})
}

// RunNow starts an unregistered schedule-shaped operation. Cron and Enabled are
// intentionally ignored; the method exists for activities that support manual
// execution before an operator creates a recurring definition.
func (s *Scheduler) RunNow(ctx flanksourceContext.Context, schedule Schedule) (*Group, error) {
	if schedule.Name == "" {
		return nil, fmt.Errorf("run name is required")
	}
	if schedule.Kind == "" {
		return nil, fmt.Errorf("run %q: kind is required", schedule.Name)
	}
	if schedule.Timeout < 0 {
		return nil, fmt.Errorf("run %q: timeout must not be negative", schedule.Name)
	}
	if schedule.Concurrency < 0 {
		return nil, fmt.Errorf("run %q: concurrency must not be negative, got %d", schedule.Name, schedule.Concurrency)
	}
	if _, ok := s.runnerFor(schedule.Kind); !ok {
		return nil, fmt.Errorf("run %q: no runner registered for kind %q", schedule.Name, schedule.Kind)
	}
	now := s.now()
	return s.start(ctx, "", nil, schedule, now, now, FireManual, map[string]string{"trigger": "manual"})
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
