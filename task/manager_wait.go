package task

import (
	"fmt"
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

// ClearTasks removes all completed tasks from the task list
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
}

// WaitSilent waits for all tasks to complete without displaying results
func WaitSilent() int {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if global.taskQueue.Empty() && global.workersActive.Load() == 0 {
			allComplete := true
			global.mu.RLock()
			for _, task := range global.tasks {
				if !task.completed.Load() {
					allComplete = false
					break
				}
			}
			global.mu.RUnlock()

			if allComplete {
				break
			}
		}

		<-ticker.C
	}

	global.stopRender()

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

// Wait waits for all tasks to complete and returns the appropriate exit code
func Wait() int {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if global.taskQueue.Empty() && global.workersActive.Load() == 0 {
			allComplete := true
			global.mu.RLock()
			for _, task := range global.tasks {
				if !task.completed.Load() {
					allComplete = false
					break
				}
			}
			global.mu.RUnlock()

			if allComplete {
				break
			}
		}

		<-ticker.C
	}

	global.stopRender()

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
	timeout := time.Second * 10
	start := time.Now()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if time.Since(start) > timeout {
			logger.Warnf("Still waiting for all tasks to complete after %v", time.Since(start))
			timeout *= 2 // Exponential backoff for next warning
		}
		if global.taskQueue.Empty() && global.workersActive.Load() == 0 {
			allComplete := true
			global.mu.RLock()
			for _, task := range global.tasks {
				if !task.completed.Load() {
					allComplete = false
					break
				}
			}
			global.mu.RUnlock()

			if allComplete {
				break
			}
		}

		<-ticker.C
	}

	// For plain render mode, force a final render
	if global.noProgress.Load() && !global.noRender.Load() {
		global.PlainRender()
	}
}
