package task

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/flanksource/commons/logger"
	"golang.org/x/sync/semaphore"
)

type Batch[T any] struct {
	Name       string
	Items      []func(logger logger.Logger) (T, error)
	MaxWorkers int
	Results    []T
	// Timeout is the maximum duration for the entire batch to complete.
	// Zero value means no timeout (infinite wait until completion or context cancellation).
	Timeout time.Duration
	// ItemTimeout is the maximum duration for each individual item to complete.
	// Zero value means no per-item timeout.
	ItemTimeout time.Duration
}

type BatchResult[T any] struct {
	Value    T
	Error    error
	Duration time.Duration
}

// WithTimeout sets the maximum duration for the entire batch to complete.
// Returns the batch for method chaining.
func (b *Batch[T]) WithTimeout(duration time.Duration) *Batch[T] {
	b.Timeout = duration
	return b
}

// WithItemTimeout sets the maximum duration for each individual item to complete.
// Returns the batch for method chaining.
func (b *Batch[T]) WithItemTimeout(duration time.Duration) *Batch[T] {
	b.ItemTimeout = duration
	return b
}

func (b *Batch[T]) tracef(t *Task, format string, args ...any) {
	if strings.Contains(os.Getenv("DEBUG"), "batch") {
		t.Tracef("BATCH %s: %s", b.Name, fmt.Sprintf(format, args...))
	}
}
func (b *Batch[T]) Run() chan BatchResult[T] {
	if b.MaxWorkers <= 0 {
		b.MaxWorkers = 4
	}

	if b.ItemTimeout <= 0 {
		b.ItemTimeout = 5 * time.Second
	}
	if b.Timeout <= 0 {
		b.Timeout = time.Duration(len(b.Items)) * b.ItemTimeout
	}
	total := len(b.Items)
	results := make(chan BatchResult[T], total)

	// Synchronization primitives to prevent race conditions
	var closeOnce sync.Once
	var wg sync.WaitGroup
	var taskMu sync.Mutex // Protects concurrent task updates
	closeResults := func() {
		closeOnce.Do(func() {
			close(results)
		})
	}

	StartTask(b.Name, func(ctx flanksourceContext.Context, t *Task) (interface{}, error) {
		b.tracef(t, "Run starting: %s, items=%d, context.Err=%v", b.Name, total, ctx.Err())

		// Create a cancellable context for batch timeout control
		batchCtx, batchCancel := context.WithCancel(context.Context(ctx))
		defer batchCancel()

		sem := semaphore.NewWeighted(int64(b.MaxWorkers))
		count := atomic.Int32{}
		done := make(chan error, 1) // Channel to signal completion from monitoring goroutine

		t.SetName(fmt.Sprintf("%s %d of %d (workers: %d, item timeout: %v, batch timeout: %v)", b.Name, 0, total, b.MaxWorkers, b.ItemTimeout, b.Timeout))
		t.SetProgress(0, total)

		// Start monitoring goroutine with timeout BEFORE processing items
		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			// Configure batch timeout: zero means no timeout (infinite)
			var timeout <-chan time.Time
			if b.Timeout > 0 {
				timeout = time.After(b.Timeout)
			} else {
				// Never fires
				timeout = make(chan time.Time)
			}

			for {
				select {
				case <-ctx.Done():
					// Task cancelled, but check if all items completed first
					b.tracef(t, "Context cancelled detected: count=%d/%d, context.Err=%v", count.Load(), total, ctx.Err())
					wg.Wait()
					finalCount := count.Load()
					b.tracef(t, "All goroutines finished after cancellation: final count=%d/%d", finalCount, total)

					if finalCount >= int32(total) {
						// All items actually completed before cancellation
						b.tracef(t, "Completed batch %s %d of %d", b.Name, finalCount, total)
						taskMu.Lock()
						t.Success()
						taskMu.Unlock()
						closeResults()
						done <- nil
					} else {
						// Genuinely cancelled mid-execution
						b.tracef(t, "Batch cancelled: %s (completed %d of %d)", b.Name, finalCount, total)
						taskMu.Lock()
						t.SetStatus(StatusCancelled)
						taskMu.Unlock()
						closeResults()
						done <- ctx.Err()
					}
					return
				case <-timeout:
					// Timeout reached - gather debugging info before cancelling
					currentCount := count.Load()
					pendingItems := int32(total) - currentCount

					t.Warnf("Batch '%s' timeout of %v reached", b.Name, b.Timeout)
					t.Warnf("  Completed: %d/%d items", currentCount, total)
					t.Warnf("  Pending: %d items", pendingItems)
					t.Warnf("  Workers: %d (max concurrency)", b.MaxWorkers)

					// Cancel batch context to stop new items from starting
					batchCancel()
					t.Debugf("Cancelling batch context and waiting for in-flight goroutines to complete...")

					// Wait for in-flight items to complete
					wg.Wait()

					finalCount := count.Load()
					completedDuringWait := finalCount - currentCount
					if completedDuringWait > 0 {
						t.Debugf("  %d items completed during wait", completedDuringWait)
					}

					if finalCount >= int32(total) {
						// All items actually completed before timeout
						b.tracef(t, "Completed batch %s %d of %d", b.Name, finalCount, total)
						taskMu.Lock()
						t.Success()
						taskMu.Unlock()
						closeResults()
						done <- nil
					} else {
						// Genuinely timed out - send error to results channel
						finalPending := int32(total) - finalCount
						t.Warnf("Batch '%s' exceeded timeout of %v", b.Name, b.Timeout)
						t.Warnf("  Final state: %d completed, %d incomplete", finalCount, finalPending)

						taskMu.Lock()
						err := fmt.Errorf("%w: batch '%s' exceeded timeout of %v (completed %d/%d items)",
							ErrBatchTimeout, b.Name, b.Timeout, finalCount, total)
						results <- BatchResult[T]{Error: err}
						_, _ = t.FailedWithError(err)
						taskMu.Unlock()
						closeResults()
						done <- err
					}
					return
				case <-ticker.C:
					currentCount := count.Load()
					if currentCount >= int32(total) {
						// All items completed
						b.tracef(t, "Completed batch %s %d of %d", b.Name, currentCount, total)
						wg.Wait()
						taskMu.Lock()
						t.Success()
						taskMu.Unlock()
						closeResults()
						done <- nil
						return
					}
					b.tracef(t, "Waiting %d of %d", currentCount, total)
				}
			}
		}()

		for i, item := range b.Items {
			b.tracef(t, "Queuing item %d of %d", i+1, total)

			// Check for context cancellation before acquiring semaphore
			if batchCtx.Err() != nil {
				b.tracef(t, "Context cancelled, stopping new items at %d of %d", i, total)
				break
			}

			if err := sem.Acquire(batchCtx, 1); err != nil {
				b.tracef(t, "Semaphore acquire failed (likely due to context cancellation): %v", err)
				break
			}
			b.tracef(t, "Acquired semaphore for item %d of %d", i+1, total)

			wg.Add(1)
			go func(item func(log logger.Logger) (T, error), itemNum int) {
				defer sem.Release(1)
				defer wg.Done()

				// Panic recovery to prevent goroutine crashes
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("panic in batch item %d: %v", itemNum, r)
						results <- BatchResult[T]{Error: fmt.Errorf("panic: %v", r)}
					}
				}()

				// Check for context cancellation before executing
				if batchCtx.Err() != nil {
					results <- BatchResult[T]{Error: batchCtx.Err()}
					newCount := count.Add(1)
					taskMu.Lock()
					t.SetName(fmt.Sprintf("%s %d of %d", b.Name, newCount, total))
					t.SetProgress(int(newCount), total)
					taskMu.Unlock()
					return
				}

				// Create per-item timeout context if ItemTimeout is set
				var itemCtx context.Context = batchCtx
				var itemCancel context.CancelFunc
				if b.ItemTimeout > 0 {
					itemCtx, itemCancel = context.WithTimeout(batchCtx, b.ItemTimeout)
					defer itemCancel()
				}

				start := time.Now()
				b.tracef(t, "Running item %d of %d", itemNum, total)

				// Check for timeout before execution
				if b.ItemTimeout > 0 && itemCtx.Err() != nil {
					duration := time.Since(start)
					t.Warnf("Item %d in batch '%s' context already done before execution", itemNum, b.Name)
					results <- BatchResult[T]{
						Error:    fmt.Errorf("%w: item %d exceeded timeout of %v", ErrItemTimeout, itemNum, b.ItemTimeout),
						Duration: duration,
					}
					newCount := count.Add(1)
					taskMu.Lock()
					t.SetName(fmt.Sprintf("%s %d of %d", b.Name, newCount, total))
					t.SetProgress(int(newCount), total)
					taskMu.Unlock()
					return
				}

				// Execute item - for now, item receives the original logger (doesn't have access to itemCtx)
				// In the future, items could be refactored to accept a context parameter
				value, err := item(t)
				duration := time.Since(start)

				// Check if item timed out during execution
				if b.ItemTimeout > 0 && itemCtx.Err() == context.DeadlineExceeded {
					t.Warnf("Item %d in batch '%s' exceeded timeout of %v", itemNum, b.Name, b.ItemTimeout)
					results <- BatchResult[T]{
						Error:    fmt.Errorf("%w: item %d exceeded timeout of %v", ErrItemTimeout, itemNum, b.ItemTimeout),
						Duration: duration,
					}
					newCount := count.Add(1)
					taskMu.Lock()
					t.SetName(fmt.Sprintf("%s %d of %d", b.Name, newCount, total))
					t.SetProgress(int(newCount), total)
					taskMu.Unlock()
					return
				}

				results <- BatchResult[T]{Value: value, Error: err, Duration: duration}
				newCount := count.Add(1)

				// t.Debugf("Item completed: count=%d/%d, duration=%v", newCount, total, duration)

				// Protect concurrent task updates with mutex
				taskMu.Lock()
				t.SetName(fmt.Sprintf("%s %d of %d", b.Name, newCount, total))
				t.SetProgress(int(newCount), total)
				taskMu.Unlock()

				// t.Infof("Finished %s %d of %d", item, newCount, total)
			}(item, i+1)
		}

		// Wait for monitoring goroutine to complete and signal status
		t.Debugf("Waiting for monitoring goroutine to signal completion")
		err := <-done
		t.Debugf("Batch.Run finished: %s, error=%v", b.Name, err)
		return nil, err
	}).SetLogLevel(logger.Trace1)

	return results
}
