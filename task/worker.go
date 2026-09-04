package task

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"

	flanksourcecontext "github.com/flanksource/commons/context"
)

// worker represents a worker goroutine that processes tasks
type worker struct {
	manager *Manager
	id      int

	// inflight is the task this worker is currently executing, nil when idle.
	// Waits read it through Manager.foregroundWorkersActive to decide whether
	// worker occupancy should block a drain: a background task holds a worker
	// but never blocks. Kept as a live pointer rather than a counter snapshot
	// because the background flag can flip mid-run (a supervised process marks
	// its bound task background only once the server generation starts).
	inflight atomic.Pointer[Task]
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

			task.mu.Lock()
			skip := task.status == StatusCancelled
			task.mu.Unlock()
			if skip {
				task.completed.Store(true)
				task.signalDone()
				if task.identity != "" {
					w.manager.tasksByIdentity.Delete(task.identity)
				}
				continue
			}

			// Check dependencies
			if !w.checkDependencies(task) {
				// Dependencies not met, re-enqueue with delay
				w.manager.taskQueue.EnqueueWithDelay(task, 50*time.Millisecond)
				continue
			}

			// Gate on group concurrency at dequeue time. If the group's permit
			// pool is saturated, do not occupy a worker slot — re-enqueue with a
			// short delay and let this worker pick up other work.
			sem := task.groupSem()
			if sem != nil && !sem.TryAcquire(1) {
				w.manager.taskQueue.EnqueueWithDelay(task, 50*time.Millisecond)
				continue
			}

			w.inflight.Store(task)
			w.manager.workersActive.Add(1)

			func() {
				// Release the group permit on ALL exit paths (success, failure,
				// retry, timeout, panic). Registered first so it runs last,
				// after the recover defer records terminal status.
				if sem != nil {
					defer sem.Release(1)
				}
				// Flip completed before the permit is released: the permit is
				// what admits the next task, and a dependent that dequeues while
				// this one is terminal but not yet completed is cancelled as
				// "dependency failed".
				defer task.completed.Store(true)
				defer func() {
					if r := recover(); r != nil {
						task.mu.Lock()
						task.status = StatusFailed
						task.err = fmt.Errorf("panic: %v", r)
						task.endTime = time.Now()
						task.mu.Unlock()
					}
				}()
				w.executeTask(task)
			}()

			w.manager.workersActive.Add(-1)

			// Clean up identity tracking
			if task.identity != "" {
				w.manager.tasksByIdentity.Delete(task.identity)
			}

			// Signal done channel for compatibility
			task.signalDone()

			// Finishing the last task in a group is what makes the group
			// terminal, and this is the only place that fact is known without
			// something happening to snapshot it. Observing it here is what
			// gives a run a durable record the moment it ends rather than
			// whenever a UI next looks.
			observeGroupTerminal(task.parent)

			// Cleared only after all post-run bookkeeping so a wait that gates
			// on foregroundWorkersActive never returns before identity cleanup
			// and the done signal have landed.
			w.inflight.Store(nil)
		}
	}
}

// checkDependencies reports whether a task may start: every dependency has to
// have finished, and none of them may have failed.
//
// A failed dependency cancels its dependents on the strength of its status
// alone, not on whether the worker has finished its bookkeeping. Deciding it
// the other way round — only while the dependency is terminal but not yet
// marked complete — makes propagation a race between the failing worker and
// whichever worker dequeues the dependent, so downstream work runs or is
// cancelled depending on which one gets there first.
//
// A cancelled dependency is different: cancelling a task is a decision about
// that task, so its dependents wait for it to finish and then run. Callers that
// want a cancellation to take the rest of a chain with it cancel the chain.
func (w *worker) checkDependencies(task *Task) bool {
	for _, dep := range task.dependencies {
		if dep == nil {
			continue
		}
		dep.mu.Lock()
		depStatus := dep.status
		dep.mu.Unlock()

		if depStatus == StatusFailed {
			task.mu.Lock()
			task.status = StatusCancelled
			task.endTime = time.Now()
			task.err = fmt.Errorf("dependency failed")
			task.completed.Store(true)
			task.mu.Unlock()
			return false
		}
		if !dep.completed.Load() {
			return false
		}
	}
	return true
}

// executeTask runs a single task
func (w *worker) executeTask(task *Task) {
	task.mu.Lock()
	task.startTime = time.Now()
	task.mu.Unlock()

	task.SetStatus(StatusRunning)

	// Apply task-specific timeout if specified
	if task.taskTimeout > 0 {
		timeoutCtx, timeoutCancel := context.WithTimeout(task.ctx, task.taskTimeout)
		defer timeoutCancel()

		// Update task context temporarily
		task.mu.Lock()
		originalCtx := task.ctx
		originalCancel := task.cancel

		// Rebuild around the timeout context but keep the original context's
		// Logger — it routes into the task's buffered logger; a bare
		// NewContext would silently reset it to the global standard logger.
		task.flanksourceCtx = flanksourcecontext.NewContext(timeoutCtx, flanksourcecontext.WithLogger(originalCtx.Logger))
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
			task.flanksourceCtx = originalCtx
			task.cancel = originalCancel
			task.mu.Unlock()
		}()
	}

	// Execute with retry logic
	w.executeWithRetry(task)

	// The task is now terminal — executeWithRetry only returns once retries are
	// exhausted, and runFunc is invoked nowhere else. Release it so the state the
	// task closure captured (parsed inputs, accumulators, large intermediate
	// outputs) is collected immediately instead of being pinned for the manager's
	// run-retention window. Completed tasks are kept only for viewing; the
	// lightweight result and snapshot carry everything that surface needs.
	task.mu.Lock()
	task.runFunc = nil
	task.mu.Unlock()
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
				// Log retry attempt to bufferedLogger
				task.getBufferedLogger().Warnf("Attempt %d failed, retrying: %v", task.retryCount, err)

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
					task.getBufferedLogger().Errorf("Failed to set error status: %v", failErr)
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
