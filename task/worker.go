package task

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	flanksourcecontext "github.com/flanksource/commons/context"
	"github.com/flanksource/commons/logger"
)

// worker represents a worker goroutine that processes tasks
type worker struct {
	manager *Manager
	id      int
}

// run is the main loop for a worker goroutine
func (w *worker) run() {
	for {
		select {
		case <-w.manager.shutdown:
			return
		default:
			// Try to dequeue a task
			task, ok := w.manager.taskQueue.Dequeue()
			if !ok {
				// No task available, sleep briefly
				time.Sleep(10 * time.Millisecond)
				continue
			}

			// Check dependencies
			if !w.checkDependencies(task) {
				// Dependencies not met, re-enqueue with delay
				w.manager.taskQueue.EnqueueWithDelay(task, 50*time.Millisecond)
				continue
			}

			// Increment active workers count
			w.manager.workersActive.Add(1)

			// Execute the task
			w.executeTask(task)

			// Decrement active workers count
			w.manager.workersActive.Add(-1)

			// Mark task as completed
			task.completed.Store(true)

			// Clean up identity tracking
			if task.identity != "" {
				w.manager.tasksByIdentity.Delete(task.identity)
			}

			// Signal done channel for compatibility
			task.signalDone()
		}
	}
}

// checkDependencies verifies all task dependencies are completed
func (w *worker) checkDependencies(task *Task) bool {
	for _, dep := range task.dependencies {
		if dep == nil {
			continue
		}
		if !dep.completed.Load() {
			// Check if dependency failed
			dep.mu.Lock()
			depStatus := dep.status
			dep.mu.Unlock()

			if depStatus == StatusFailed || depStatus == StatusCancelled {
				// Dependency failed, mark this task as canceled
				task.mu.Lock()
				task.status = StatusCancelled
				task.endTime = time.Now()
				task.err = fmt.Errorf("dependency failed")
				task.completed.Store(true)
				task.mu.Unlock()
				return false
			}
			// Dependency not yet complete
			return false
		}
	}
	return true
}

// executeTask runs a single task
func (w *worker) executeTask(task *Task) {
	task.startTime = time.Now()

	task.SetStatus(StatusRunning)

	// Apply task-specific timeout if specified
	if task.taskTimeout > 0 {
		timeoutCtx, timeoutCancel := context.WithTimeout(task.ctx, task.taskTimeout)
		defer timeoutCancel()

		// Update task context temporarily
		task.mu.Lock()
		originalCtx := task.ctx
		originalCancel := task.cancel

		task.flanksourceCtx = flanksourcecontext.NewContext(timeoutCtx)
		task.ctx = task.flanksourceCtx
		task.cancel = func() {
			timeoutCancel()
			originalCancel()
		}
		task.mu.Unlock()

		// Restore original context after execution
		defer func() {
			task.mu.Lock()
			task.ctx = originalCtx
			task.flanksourceCtx = flanksourcecontext.NewContext(originalCtx)
			task.cancel = originalCancel
			task.mu.Unlock()
		}()
	}

	// Execute with retry logic
	w.executeWithRetry(task)
}

// executeWithRetry handles task execution with exponential backoff retry
func (w *worker) executeWithRetry(task *Task) {
	for {
		// Check if task has a function to run
		if task.runFunc == nil {
			// No function, mark as success
			task.SetStatus(StatusSuccess)
			return
		}

		// Execute the task function
		err := task.runFunc(task.flanksourceCtx, task)

		if task.status != StatusRunning {
			return
		}

		if err != nil {
			// Check if error is retryable
			shouldRetry := shouldRetryError(err, task.retryConfig)

			if shouldRetry && task.retryCount < task.retryConfig.MaxRetries {
				task.retryCount++
				task.logs = append(task.logs, LogEntry{
					Level:   logger.Warn,
					Message: fmt.Sprintf("Attempt %d failed, retrying: %v", task.retryCount, err),
					Time:    time.Now(),
				})
				task.mu.Unlock()

				// Calculate backoff delay
				delay := calculateBackoffDelay(task.retryCount, task.retryConfig)

				// Wait for delay or cancellation
				select {
				case <-time.After(delay):
					continue // Retry
				case <-task.ctx.Done():
					task.SetStatus(StatusCancelled)
					return
				}
			} else {
				if _, failErr := task.FailedWithError(err); failErr != nil {
					// Log error but continue - task is already in failed state
					task.logs = append(task.logs, LogEntry{
						Level:   logger.Error,
						Message: fmt.Sprintf("Failed to set error status: %v", failErr),
						Time:    time.Now(),
					})
				}
				return
			}
		} else {
			task.Success()
			return
		}
	}
}

// shouldRetryError checks if an error should trigger a retry
func shouldRetryError(err error, config RetryConfig) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	for _, pattern := range config.RetryableErrors {
		if strings.Contains(errMsg, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// calculateBackoffDelay calculates the delay for the next retry with exponential backoff and jitter
func calculateBackoffDelay(retryCount int, config RetryConfig) time.Duration {
	// Calculate exponential backoff
	delay := float64(config.BaseDelay) * math.Pow(config.BackoffFactor, float64(retryCount-1))

	// Apply maximum delay cap
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	// Add jitter to prevent thundering herd
	jitter := delay * config.JitterFactor * (rand.Float64() - 0.5) * 2
	finalDelay := delay + jitter

	// Ensure delay is never negative
	if finalDelay < 0 {
		finalDelay = float64(config.BaseDelay)
	}

	return time.Duration(finalDelay)
}
