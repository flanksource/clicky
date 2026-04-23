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
	assert.Equal(t, before, SnapshotTask(task, "group").ID)
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
