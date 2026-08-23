package task

import (
	"testing"
	"time"

	flanksourcecontext "github.com/flanksource/commons/context"
)

// runningTask builds a task registered on tm that is Running and will never
// complete — the shape of a supervised long-lived server process.
func runningTask(tm *Manager, name string, background bool) *Task {
	t := tm.newTask(name)
	t.SetBackground(background)
	t.mu.Lock()
	t.status = StatusRunning
	t.startTime = time.Now()
	t.mu.Unlock()
	tm.mu.Lock()
	tm.tasks = append(tm.tasks, t)
	tm.mu.Unlock()
	return t
}

// awaitReturned reports whether tm.awaitAll returned within limit.
func awaitReturned(tm *Manager, limit time.Duration) bool {
	done := make(chan struct{})
	go func() {
		tm.awaitAll()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(limit):
		return false
	}
}

// TestAwaitAllSkipsBackgroundTask is the regression guard for the `gavel pr
// status --ai-fix` hang: a supervised agent-provider process registers a global
// task that stays Running for the whole session, and the commit hook running
// inside that session drains global tasks. Before background tasks were skipped
// the two deadlocked and the wait warned forever instead of returning.
func TestAwaitAllSkipsBackgroundTask(t *testing.T) {
	tm := newTestManager(1)
	runningTask(tm, "claude-agent/node_modules/.bin/tsx", true)

	if !awaitReturned(tm, 2*time.Second) {
		t.Fatal("awaitAll blocked on a background task; a long-lived server must never block a drain")
	}
}

// TestAwaitAllStillBlocksOnForegroundTask guards the opt-in default: a
// supervised process that IS the work (`gavel proc run`) must keep blocking.
func TestAwaitAllStillBlocksOnForegroundTask(t *testing.T) {
	tm := newTestManager(1)
	blocking := runningTask(tm, "run tests", false)

	if awaitReturned(tm, 200*time.Millisecond) {
		t.Fatal("awaitAll returned while an ordinary task was still running")
	}

	blocking.completed.Store(true)
	if !awaitReturned(tm, 2*time.Second) {
		t.Fatal("awaitAll did not return after the blocking task completed")
	}
}

func TestTasksDrainedIgnoresBackgroundOnly(t *testing.T) {
	tm := newTestManager(1)
	server := runningTask(tm, "agent server", true)
	work := runningTask(tm, "commit", false)

	if tasksDrained([]*Task{server, work}) {
		t.Fatal("tasksDrained reported drained while a foreground task was incomplete")
	}

	work.completed.Store(true)
	if !tasksDrained([]*Task{server, work}) {
		t.Fatal("tasksDrained blocked on a background task")
	}
}

// scheduledTask enqueues a task through the manager's priority queue so a
// worker goroutine executes it — unlike runningTask, which fakes a running
// state without ever occupying a worker.
func scheduledTask(tm *Manager, name string, fn func(flanksourcecontext.Context, *Task) error) *Task {
	t := tm.newTask(name)
	t.runFunc = fn
	return tm.enqueue(t)
}

// TestAwaitAllSkipsScheduledBackgroundWorker guards the worker-occupancy side
// of the background contract: tasksDrained already skips a background task,
// but the server also holds a worker for its whole life, so a wait gating on
// raw worker occupancy blocked forever anyway.
func TestAwaitAllSkipsScheduledBackgroundWorker(t *testing.T) {
	tm := newTestManager(2)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	server := tm.newTask("agent server")
	server.SetBackground(true)
	server.runFunc = func(flanksourcecontext.Context, *Task) error {
		<-release
		return nil
	}
	tm.enqueue(server)

	work := scheduledTask(tm, "commit", func(flanksourcecontext.Context, *Task) error {
		return nil
	})

	if !awaitReturned(tm, 2*time.Second) {
		t.Fatal("awaitAll blocked on a scheduled background task occupying a worker")
	}
	if !work.completed.Load() {
		t.Fatal("awaitAll returned before the foreground task completed")
	}
	if server.completed.Load() {
		t.Fatal("background task finished on its own; the test no longer exercises a long-lived server")
	}
}

// TestAwaitAllSkipsTaskMarkedBackgroundMidRun mirrors exec.RunSupervisedAsTask:
// the task starts as ordinary scheduled work and is only marked background from
// inside its own run, once the supervised server generation is up. A snapshot
// of the flag taken at dequeue time would miss the flip and block the wait.
func TestAwaitAllSkipsTaskMarkedBackgroundMidRun(t *testing.T) {
	tm := newTestManager(2)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	server := scheduledTask(tm, "supervised server", func(_ flanksourcecontext.Context, current *Task) error {
		current.SetBackground(true)
		<-release
		return nil
	})

	work := scheduledTask(tm, "commit", func(flanksourcecontext.Context, *Task) error {
		return nil
	})

	if !awaitReturned(tm, 2*time.Second) {
		t.Fatal("awaitAll blocked on a worker whose task turned background mid-run")
	}
	if !work.completed.Load() {
		t.Fatal("awaitAll returned before the foreground task completed")
	}
	if server.completed.Load() {
		t.Fatal("background task finished on its own; the test no longer exercises a long-lived server")
	}
}

// TestIncompleteTaskNamesOmitsBackground keeps the diagnostic honest: naming a
// task that cannot be the cause is what made the original hang read as if the
// agent process were the thing to wait for.
func TestIncompleteTaskNamesOmitsBackground(t *testing.T) {
	tm := newTestManager(1)
	runningTask(tm, "agent server", true)
	runningTask(tm, "commit", false)

	names := tm.incompleteTaskNames()
	if len(names) != 1 {
		t.Fatalf("incompleteTaskNames() = %v, want only the foreground task", names)
	}
	if got := names[0]; got != "commit (running)" {
		t.Fatalf("incompleteTaskNames() = %q, want %q", got, "commit (running)")
	}
}
