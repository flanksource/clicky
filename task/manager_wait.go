package task

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/commons/logger"
)

// Run starts all tasks and waits for completion
func (tm *Manager) Run() error {
	Wait()

	// Check if any tasks failed
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	for _, task := range tm.tasks {
		if task.err != nil {
			return fmt.Errorf("task %s failed: %w", task.name, task.err)
		}
	}
	return nil
}

// CancelAll cancels all running tasks and groups
func CancelAll() {
	global.mu.RLock()
	defer global.mu.RUnlock()
	for _, task := range global.tasks {
		task.Cancel()
	}
	for _, group := range global.groups {
		group.Cancel()
	}
}

// StopTask cancels a specific pending or running task by immutable ID.
func StopTask(id string) bool {
	if global == nil || id == "" {
		return false
	}

	global.mu.RLock()
	tasks := make([]*Task, len(global.tasks))
	copy(tasks, global.tasks)
	global.mu.RUnlock()

	for _, task := range tasks {
		if task == nil || task.ID() != id {
			continue
		}

		task.mu.Lock()
		runnable := task.status == StatusPending || task.status == StatusRunning
		task.mu.Unlock()
		if !runnable {
			return false
		}

		task.Cancel()
		return true
	}

	return false
}

// ClearTasks removes all completed tasks (and groups) from the registry,
// keeping only those still pending or running. Groups must be pruned too:
// SnapshotAll iterates global.groups, so a leftover completed group would keep
// surfacing its (now-removed) tasks — e.g. a stale "Running tests" group whose
// task IDs no longer resolve, causing StopTask lookups to fail.
func ClearTasks() {
	global.mu.Lock()
	defer global.mu.Unlock()

	var activeTasks []*Task
	for _, task := range global.tasks {
		task.mu.Lock()
		status := task.status
		task.mu.Unlock()

		if status == StatusPending || status == StatusRunning {
			activeTasks = append(activeTasks, task)
		}
	}
	global.tasks = activeTasks

	var activeGroups []*Group
	for _, group := range global.groups {
		if s := group.Status(); s == StatusPending || s == StatusRunning {
			activeGroups = append(activeGroups, group)
		}
	}
	global.groups = activeGroups
}

// awaitAllTasks blocks until the queue is drained, no worker is running
// foreground work, and every registered non-background task has flipped
// completed. Background tasks are long-lived servers that outlive the wait by
// contract and are skipped — waiting on one deadlocks (see Task.SetBackground).
// Worker occupancy is judged per in-flight task (foregroundWorkersActive), not
// by the raw workersActive counter: a background server scheduled through the
// queue holds a worker for its whole life and must not block the drain.
//
// The wait is deliberately unbounded: a legitimately long run (a 30-minute test
// suite) must not be killed by a timer. A task that never completes is instead
// named on a doubling interval rather than waited on in silence — a mute poll
// here is what let a single wedged task leave gavel processes ticking at 10ms
// for days with nothing in the logs and no open file descriptors to point at
// the cause.
func awaitAllTasks() {
	global.awaitAll()
}

func (tm *Manager) awaitAll() {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	start := time.Now()
	warnAfter := waitForWarnAfter

	for {
		if tm.taskQueue.Empty() && tm.foregroundWorkersActive() == 0 && tm.allTasksCompleted() {
			return
		}

		if waited := time.Since(start); waited >= warnAfter {
			logger.Warnf("Still waiting after %s for: %s",
				waited.Round(time.Second), strings.Join(tm.incompleteTaskNames(), ", "))
			warnAfter *= 2
		}

		<-ticker.C
	}
}

// foregroundWorkersActive counts workers whose in-flight task a wait is
// entitled to drain. Judged from the task's live background flag rather than a
// snapshot taken at dequeue, because the flag can flip mid-run: a supervised
// process scheduled through the queue (exec.RunSupervisedAsTask) marks its
// bound task background only once the server generation starts. tm.workers is
// populated once at construction and never mutated, so it is read without mu.
func (tm *Manager) foregroundWorkersActive() int {
	active := 0
	for _, w := range tm.workers {
		if t := w.inflight.Load(); t != nil && !t.IsBackground() {
			active++
		}
	}
	return active
}

func (tm *Manager) allTasksCompleted() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tasksDrained(tm.tasks)
}

// tasksDrained reports whether every task a wait is entitled to block on has
// completed. Background tasks are skipped: they are long-lived servers that by
// contract outlive the wait, so counting them deadlocks it — the wait blocks the
// work that would shut the server down, and the server keeps the wait from
// returning. See Task.SetBackground.
func tasksDrained(tasks []*Task) bool {
	for _, task := range tasks {
		if task.IsBackground() {
			continue
		}
		if !task.completed.Load() {
			return false
		}
	}
	return true
}

// incompleteTaskNames names the tasks still blocking a wait, for diagnostics.
func (tm *Manager) incompleteTaskNames() []string {
	tm.mu.RLock()
	tasks := make([]*Task, len(tm.tasks))
	copy(tasks, tm.tasks)
	tm.mu.RUnlock()

	var names []string
	for _, task := range tasks {
		// Background tasks never block the wait, so naming one would point the
		// reader at a task that is not the cause.
		if task.IsBackground() {
			continue
		}
		if task.completed.Load() {
			continue
		}
		names = append(names, fmt.Sprintf("%s (%s)", task.Name(), task.Status()))
	}
	return names
}

// WaitSilent waits for all tasks to complete without displaying results.
// It stops the renderer and stops output capture (flushing any output
// buffered by StartCapturingOutput to the restored streams).
func WaitSilent() int {
	awaitAllTasks()

	global.stopRender()
	global.StopCapturingOutput()

	global.mu.RLock()
	tasks := global.tasks
	global.mu.RUnlock()

	for _, task := range tasks {
		task.mu.Lock()
		status := task.status
		task.mu.Unlock()

		switch status {
		case StatusFailed, StatusCancelled:
			return 1
		}
	}

	return 0
}

// Wait waits for all tasks to complete and returns the appropriate exit code.
// It stops the renderer and stops output capture (flushing any output
// buffered by StartCapturingOutput to the restored streams).
func Wait() int {
	awaitAllTasks()

	global.stopRender()
	global.StopCapturingOutput()

	var failed, canceled int

	global.mu.RLock()
	tasks := global.tasks
	global.mu.RUnlock()

	for _, task := range tasks {
		task.mu.Lock()
		status := task.status
		task.mu.Unlock()

		switch status {
		case StatusFailed:
			failed++
		case StatusCancelled:
			canceled++
		}
	}

	if failed+canceled > 0 {
		return 1
	}
	return 0
}

// Debug returns debug information about the task manager
func Debug() string {
	global.mu.RLock()
	defer global.mu.RUnlock()

	var result string
	result += fmt.Sprintf("Task Manager: {no-color=%v, no-progress=%v, no-render=%v, workers=%v}\n", global.noColor.Load(), global.noProgress.Load(), global.noRender.Load(), global.workersActive.Load())
	result += fmt.Sprintf("  Total Tasks: %d\n", len(global.tasks))
	result += fmt.Sprintf("  Active Workers: %d\n", global.workersActive.Load())
	result += "  Task Details:\n"
	for _, task := range global.tasks {
		task.mu.Lock()
		result += fmt.Sprintf("    - %s: %v\n", task.name, task.status)
		task.mu.Unlock()
	}
	return result
}

// WaitForAllTasks waits for all global tasks to complete and forces a final render
func WaitForAllTasks() {
	awaitAllTasks()

	// For plain render mode, force a final render
	if global.noProgress.Load() && !global.noRender.Load() {
		global.PlainRender()
	}
}
