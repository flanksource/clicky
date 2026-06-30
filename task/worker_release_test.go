package task

import (
	"runtime"
	"testing"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
)

type heavyPayload struct{ buf []byte }

// enqueueClosurePinning starts a task whose closure captures p (and nothing the
// test stack still holds). It is a helper so that, once it returns, the only
// live reference to p is the task's runFunc closure — letting the test prove
// that closure (and p) is released when the task completes.
func enqueueClosurePinning(m *Manager, p *heavyPayload) *Task {
	task := m.newTask("heavy")
	task.runFunc = func(flanksourceContext.Context, *Task) error {
		runtime.KeepAlive(p)
		return nil
	}
	m.enqueue(task)
	return task
}

// TestRunFuncReleasedAfterCompletion verifies the worker drops a completed
// task's runFunc, so the (potentially large) state the task closure captured is
// not pinned for the manager's run-retention window. Without the release the
// finalizer never fires and the test fails.
func TestRunFuncReleasedAfterCompletion(t *testing.T) {
	manager := newTestManager(1)
	manager.noRender.Store(true)

	collected := make(chan struct{})
	captured := &heavyPayload{buf: make([]byte, 8<<20)} // 8 MiB
	runtime.SetFinalizer(captured, func(*heavyPayload) { close(collected) })

	task := enqueueClosurePinning(manager, captured)
	captured = nil // drop the test's own reference; only the closure holds it now

	select {
	case <-task.doneChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for task completion")
	}

	task.mu.Lock()
	released := task.runFunc == nil
	task.mu.Unlock()
	if !released {
		t.Fatal("runFunc was not released after the task completed")
	}

	for i := 0; i < 50; i++ {
		runtime.GC()
		select {
		case <-collected:
			return // payload was finalized — the closure is no longer pinned
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Fatal("captured payload was not collected; a completed task still pins its closure state")
}
