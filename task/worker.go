package task

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"
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
			skip := task.status == StatusCancelled || task.ctx.Err() != nil
			task.mu.Unlock()
			if skip {
				task.Cancel()
				task.markCompleted()
				task.signalDone()
				w.manager.releaseIdentity(task)
				// Retiring a cancelled task can be what makes its group
				// terminal just as much as running one can, so the run is
				// handed to the store here too. See the tail of the loop.
				observeGroupTerminal(task.parent)
				continue
			}

			// Check dependencies
			if !w.checkDependencies(task) {
				// Dependencies not met, re-enqueue with delay
				w.manager.taskQueue.EnqueueWithDelay(task, 50*time.Millisecond)
				continue
			}

			// Gate on group concurrency at dequeue time. If the group's permit
			// pool is saturated, or a member added before this one is still
			// waiting for a permit, do not occupy a worker slot — re-enqueue with
			// a short delay and let this worker pick up other work.
			sem := task.groupSem()
			if sem != nil && !task.parent.admit(task) {
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
				defer task.markCompleted()
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

			w.manager.releaseIdentity(task)

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
	if task.status == StatusCancelled || task.ctx.Err() != nil {
		task.mu.Unlock()
		task.Cancel()
		return
	}
	task.startTime = time.Now()
	task.status = StatusRunning
	task.mu.Unlock()
	task.dirty.Store(true)

	defer task.armTimeout()()

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

// armTimeout starts the task's timeout budget. It runs once the task has been
// scheduled — dependencies satisfied, group permit held — so time spent queued
// never counts against the budget. One budget covers every retry attempt and
// the backoff between them. The returned func releases the timer.
func (t *Task) armTimeout() context.CancelFunc {
	if t.timeout <= 0 {
		return func() {}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.timeoutErr = fmt.Errorf("task %q timed out after %s: %w", t.name, t.timeout, context.DeadlineExceeded)
	// Swap only the embedded context so the Logger (routed into the task's
	// buffered logger), tracer and debug/trace hooks carry over.
	var timeoutCancel context.CancelFunc
	t.flanksourceCtx.Context, timeoutCancel = context.WithTimeoutCause(t.flanksourceCtx.Context, t.timeout, t.timeoutErr)
	t.ctx = t.flanksourceCtx
	parentCancel := t.cancel
	t.cancel = func() {
		timeoutCancel()
		if parentCancel != nil {
			parentCancel()
		}
	}
	return timeoutCancel
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

		task.mu.Lock()
		status := task.status
		cancelled := task.ctx.Err() != nil
		task.mu.Unlock()
		if status != StatusRunning {
			return
		}
		if cancelled && err != nil {
			task.Cancel()
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
					task.Cancel()
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
