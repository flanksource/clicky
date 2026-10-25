package task

import (
	"sync/atomic"
	"testing"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotTaskIDIsStableAcrossRename(t *testing.T) {
	tm := newTestManager(1)
	task := tm.newTask("first-name", WithIdentity("dedupe-key"))

	require.NotEmpty(t, task.ID())
	assert.NotEqual(t, "dedupe-key", task.ID())

	before := task.ID()
	task.SetName("second-name")
	after := task.ID()

	assert.Equal(t, before, after)
	assert.Equal(t, before, SnapshotTask(task, nil).ID)
}

func TestStopTaskCancelsRunningTaskByID(t *testing.T) {
	originalGlobal := global
	global = newTestManager(1)
	t.Cleanup(func() {
		global.stopRender()
		global = originalGlobal
	})

	started := make(chan struct{})
	task := StartTask[string]("blocking", func(ctx flanksourceContext.Context, _ *Task) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	})

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("task did not start")
	}

	require.True(t, StopTask(task.ID()))

	wait := task.WaitFor()
	assert.Equal(t, StatusCancelled, wait.Status)
}

func TestStopTaskCancelsPendingTaskByID(t *testing.T) {
	originalGlobal := global
	global = newTestManager(1)
	t.Cleanup(func() {
		global.stopRender()
		global = originalGlobal
	})

	started := make(chan struct{})
	release := make(chan struct{})
	first := StartTask[string]("first", func(ctx flanksourceContext.Context, _ *Task) (string, error) {
		close(started)
		select {
		case <-release:
			return "done", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first task did not start")
	}

	var secondRan atomic.Bool
	second := StartTask[string]("second", func(flanksourceContext.Context, *Task) (string, error) {
		secondRan.Store(true)
		return "done", nil
	})

	require.True(t, StopTask(second.ID()))
	close(release)

	first.WaitFor()
	wait := second.WaitFor()
	assert.Equal(t, StatusCancelled, wait.Status)
	assert.False(t, secondRan.Load())
}

func TestStopTaskReturnsFalseForUnknownOrCompletedTask(t *testing.T) {
	originalGlobal := global
	global = newTestManager(1)
	t.Cleanup(func() {
		global.stopRender()
		global = originalGlobal
	})

	task := StartTask[string]("done", func(flanksourceContext.Context, *Task) (string, error) {
		return "ok", nil
	})
	wait := task.WaitFor()
	require.Equal(t, StatusSuccess, wait.Status)

	assert.False(t, StopTask(""))
	assert.False(t, StopTask("missing"))
	assert.False(t, StopTask(task.ID()))
}

// Regression: StatusWarning must be terminal — endTime is frozen so the
// rendered duration stops ticking and the task context is cancelled.
// Before this fix, Warning() left endTime zero, so getDuration() kept
// returning time.Now()-startTime on every render, producing an ever-growing
// elapsed counter even after the task's goroutine had returned.
func TestSetStatus_TerminalStatesFreezeEndTime(t *testing.T) {
	cases := []struct {
		name   string
		mark   func(*Task)
		status Status
	}{
		{"Success", func(tk *Task) { tk.Success() }, StatusSuccess},
		{"Failed", func(tk *Task) { tk.Failed() }, StatusFailed},
		{"Warning", func(tk *Task) { tk.Warning() }, StatusWarning},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tm := newTestManager(1)
			t.Cleanup(func() { close(tm.shutdown) })

			tk := tm.newTask(tc.name + "-task")
			tk.SetStatus(StatusRunning)

			cancelCh := tk.ctx.Done()

			tc.mark(tk)

			tk.mu.Lock()
			end := tk.endTime
			tk.mu.Unlock()
			if end.IsZero() {
				t.Fatalf("%s: endTime is zero — task not marked terminal", tc.name)
			}

			select {
			case <-cancelCh:
			default:
				t.Fatalf("%s: task ctx not cancelled — SetStatus skipped the terminal cleanup branch", tc.name)
			}

			d1 := tk.WaitTime()
			time.Sleep(20 * time.Millisecond)
			d2 := tk.WaitTime()
			if d1 != d2 {
				t.Fatalf("%s: WaitTime kept ticking after terminal status: %v -> %v", tc.name, d1, d2)
			}

			tk.mu.Lock()
			rendered1 := tk.getDuration()
			tk.mu.Unlock()
			time.Sleep(20 * time.Millisecond)
			tk.mu.Lock()
			rendered2 := tk.getDuration()
			tk.mu.Unlock()
			if rendered1 != rendered2 {
				t.Fatalf("%s: getDuration() kept ticking after terminal status: %q -> %q", tc.name, rendered1, rendered2)
			}
		})
	}
}
