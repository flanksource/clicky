package task

import (
	"errors"
	"strings"
	"testing"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
)

// TestCancelledDependencyDoesNotCancelDependent covers dropping one item from a
// dependency chain — a queue where each entry waits for the previous one, and
// the user removes an entry that has not started yet. Removing it says nothing
// about the entries behind it: they must still run. checkDependencies cancels a
// dependent whose dependency is Cancelled and not yet completed, so a task
// cancelled while pending has to record its completion at cancel time; nothing
// will ever run it.
func TestCancelledDependencyDoesNotCancelDependent(t *testing.T) {
	manager := newTestManager(1)
	manager.noRender.Store(true)

	dropped := manager.newTask("dropped")
	dropped.Cancel()

	ran := make(chan struct{})
	dependent := manager.newTask("dependent")
	dependent.dependencies = []*Task{dropped}
	dependent.runFunc = func(flanksourceContext.Context, *Task) error {
		close(ran)
		return nil
	}
	manager.enqueue(dependent)

	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatalf("dependent never ran; status %s, err %v", dependent.Status(), dependent.Error())
	}
}

// TestCancelledPendingTaskIsComplete pins the flag the dependency check reads:
// a task cancelled before it started is finished, not in flight.
func TestCancelledPendingTaskIsComplete(t *testing.T) {
	manager := newTestManager(1)
	manager.noRender.Store(true)

	task := manager.newTask("dropped")
	task.Cancel()

	if !task.completed.Load() {
		t.Fatal("a task cancelled while pending should be marked completed; it will never run")
	}
	if task.Status() != StatusCancelled {
		t.Fatalf("expected status %s, got %s", StatusCancelled, task.Status())
	}
}

// TestFailedDependencyCancelsDependentAfterCompletion is the other half of the
// contract: a dependency that failed stops its dependents whenever they are
// dequeued, including long after it finished. Reading the failure off the
// completion flag instead of the status made propagation a coin toss between
// the failing worker and the one picking up the dependent.
func TestFailedDependencyCancelsDependentAfterCompletion(t *testing.T) {
	manager := newTestManager(1)
	manager.noRender.Store(true)

	broken := manager.newTask("broken")
	broken.runFunc = func(flanksourceContext.Context, *Task) error {
		return errors.New("the commit did not apply")
	}
	manager.enqueue(broken)
	waitForStatus(t, broken, StatusFailed)

	ran := make(chan struct{})
	dependent := manager.newTask("dependent")
	dependent.dependencies = []*Task{broken}
	dependent.runFunc = func(flanksourceContext.Context, *Task) error {
		close(ran)
		return nil
	}
	manager.enqueue(dependent)
	waitForStatus(t, dependent, StatusCancelled)

	select {
	case <-ran:
		t.Fatal("dependent ran even though its dependency failed")
	default:
	}
	if err := dependent.Error(); err == nil || !strings.Contains(err.Error(), "dependency failed") {
		t.Fatalf("expected a dependency failure, got %v", err)
	}
}

// TestCancelledRunningDependencyDoesNotCancelDependent is the pending-cancel
// case once the task has started: the dependent waits for the cancelled task to
// unwind and then runs, rather than being cancelled because it happened to be
// dequeued while the cancelled task was still returning.
func TestCancelledRunningDependencyDoesNotCancelDependent(t *testing.T) {
	manager := newTestManager(2)
	manager.noRender.Store(true)

	started := make(chan struct{})
	stopped := manager.newTask("stopped")
	stopped.runFunc = func(ctx flanksourceContext.Context, _ *Task) error {
		close(started)
		<-ctx.Done()
		// Unwind slowly, so the dependent is certain to be dequeued while this
		// task is cancelled but has not finished.
		time.Sleep(300 * time.Millisecond)
		return ctx.Err()
	}
	manager.enqueue(stopped)

	ran := make(chan struct{})
	dependent := manager.newTask("dependent")
	dependent.dependencies = []*Task{stopped}
	dependent.runFunc = func(flanksourceContext.Context, *Task) error {
		close(ran)
		return nil
	}
	manager.enqueue(dependent)

	<-started
	stopped.Cancel()

	select {
	case <-ran:
	case <-time.After(3 * time.Second):
		t.Fatalf("dependent never ran; status %s, err %v", dependent.Status(), dependent.Error())
	}
}

func waitForStatus(t *testing.T, task *Task, want Status) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if task.Status() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("task %s never reached %s; status %s, err %v", task.Name(), want, task.Status(), task.Error())
}
