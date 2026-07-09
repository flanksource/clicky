package task

import (
	"strings"
	"testing"
	"time"

	"github.com/flanksource/clicky/text"
)

// addDirtyLoggedTask registers a completed, dirty task directly on the
// manager's task list so PlainRender can be driven manually without workers.
func addDirtyLoggedTask(tm *Manager, name string) *Task {
	task := tm.newTask(name)
	task.SetStatus(StatusSuccess)
	task.completed.Store(true)
	task.dirty.Store(true)
	tm.mu.Lock()
	tm.tasks = append(tm.tasks, task)
	tm.mu.Unlock()
	return task
}

// F11: PlainRender must not destroy log history — after a tick prints a dirty
// task, snapshots (SSE/registry) and the final tree still see every buffered
// entry.
func TestPlainRenderPreservesLogHistoryInSnapshot(t *testing.T) {
	tm, _ := newRestartManager(t)
	tm.noProgress.Store(true) // drive PlainRender manually, no loop

	task := addDirtyLoggedTask(tm, "history-task")
	task.Warnf("history-log-1")
	task.Warnf("history-log-2")

	tm.PlainRender()

	snap := SnapshotTask(task, nil)
	if len(snap.Logs) != 2 {
		t.Fatalf("PlainRender must preserve buffered logs for snapshots; got %d entries, want 2 (%+v)", len(snap.Logs), snap.Logs)
	}
	for i, want := range []string{"history-log-1", "history-log-2"} {
		if snap.Logs[i].Message != want {
			t.Errorf("snapshot log %d = %q, want %q", i, snap.Logs[i].Message, want)
		}
	}
}

// F11(2): incremental plain output is preserved — a log emitted between two
// PlainRender ticks prints exactly once, not on every subsequent tick.
func TestPlainRenderEmitsEachLogOnce(t *testing.T) {
	tm, capture := newRestartManager(t)
	tm.noProgress.Store(true)

	task := addDirtyLoggedTask(tm, "incremental-task")
	task.Warnf("incremental-log-first")
	tm.PlainRender()

	task.Warnf("incremental-log-second")
	task.dirty.Store(true)
	tm.PlainRender()

	waitForRenderOutput(t, capture, "incremental-log-second")
	stripped := text.StripANSI(capture.String())
	for _, want := range []string{"incremental-log-first", "incremental-log-second"} {
		if got := strings.Count(stripped, want); got != 1 {
			t.Errorf("log %q must print exactly once across ticks, got %d occurrences; output:\n%s", want, got, stripped)
		}
	}
}

// F8: when the plain render loop ran, the final output after stopRender is the
// loop's own last PlainRender plus ONE gray one-line summary — not a second
// copy of every task via the full tree.
func TestLoopModeFinalOutputNoDuplicateAndSummary(t *testing.T) {
	tm, capture := newRestartManager(t)

	runRestartTask(t, tm, "loop-dedupe-task")
	tm.stopRender()

	waitForRenderOutput(t, capture, "1 task: 1 ok")

	stripped := text.StripANSI(capture.String())
	successLines := 0
	for _, line := range strings.Split(stripped, "\n") {
		if strings.Contains(line, "loop-dedupe-task") && strings.Contains(line, "✓") {
			successLines++
		}
	}
	if successLines != 1 {
		t.Errorf("completed task must print exactly once in loop mode, got %d success lines; output:\n%s", successLines, stripped)
	}
	if got := strings.Count(stripped, "1 task: 1 ok"); got != 1 {
		t.Errorf("expected exactly one summary line, got %d; output:\n%s", got, stripped)
	}
}

// F8(2): when no loop ran (noProgress/CI), stopRender prints the full tree
// once; a repeat stopRender with nothing newly enqueued prints nothing.
func TestNoLoopFinalRenderIdempotent(t *testing.T) {
	tm, capture := newRestartManager(t)
	tm.noProgress.Store(true)

	runRestartTask(t, tm, "noloop-task")

	tm.stopRender()
	waitForRenderOutput(t, capture, "noloop-task")

	tm.stopRender()
	time.Sleep(100 * time.Millisecond) // let any (wrong) duplicate output drain into capture

	stripped := text.StripANSI(capture.String())
	if got := strings.Count(stripped, "noloop-task"); got != 1 {
		t.Errorf("no-loop final tree must print exactly once across repeated stopRender, got %d; output:\n%s", got, stripped)
	}
}
