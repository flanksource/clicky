package task

import (
	"fmt"
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
}

type BatchResult[T any] struct {
	Value    T
	Error    error
	Duration time.Duration
}

func (b *Batch[T]) Run() chan BatchResult[T] {
	if b.MaxWorkers <= 0 {
		b.MaxWorkers = 3
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
		t.Debugf("Batch.Run starting: %s, items=%d, context.Err=%v", b.Name, total, ctx.Err())

		sem := semaphore.NewWeighted(int64(b.MaxWorkers))
		count := atomic.Int32{}
		done := make(chan error, 1) // Channel to signal completion from monitoring goroutine

		t.SetName(fmt.Sprintf("%s %d of %d (concurrency:%d)", b.Name, 0, total, b.MaxWorkers))
		t.SetProgress(0, total)

		for i, item := range b.Items {
			t.Infof("Queuing %v %d of %d", item, i+1, total)

			// Check for context cancellation before acquiring semaphore
			if ctx.Err() != nil {
				closeResults()
				return nil, ctx.Err()
			}

			if err := sem.Acquire(ctx, 1); err != nil {
				t.Errorf("failed to acquire semaphore: %v", err)
				closeResults()
				return nil, err
			}
			t.Infof("Acquired semaphore %v %d of %d", item, i+1, total)

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
				if ctx.Err() != nil {
					results <- BatchResult[T]{Error: ctx.Err()}
					return
				}

				start := time.Now()
				t.Infof("Running %v %d of %d", item, itemNum, total)

				value, err := item(t)
				duration := time.Since(start)
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

		// Monitoring goroutine with timeout
		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			timeout := time.After(5 * time.Hour) // 5 hour timeout for batch completion

			for {
				select {
				case <-ctx.Done():
					// Task cancelled, but check if all items completed first
					t.Infof("Context cancelled detected: count=%d/%d, context.Err=%v", count.Load(), total, ctx.Err())
					wg.Wait()
					finalCount := count.Load()
					t.Infof("All goroutines finished after cancellation: final count=%d/%d", finalCount, total)

					if finalCount >= int32(total) {
						// All items actually completed before cancellation
						t.Infof("Completed batch %s %d of %d", b.Name, finalCount, total)
						taskMu.Lock()
						t.Success()
						taskMu.Unlock()
						closeResults()
						done <- nil
					} else {
						// Genuinely cancelled mid-execution
						t.Infof("Batch cancelled: %s (completed %d of %d)", b.Name, finalCount, total)
						taskMu.Lock()
						t.SetStatus(StatusCancelled)
						taskMu.Unlock()
						closeResults()
						done <- ctx.Err()
					}
					return
				case <-timeout:
					// Timeout reached, but check if all items completed
					t.Infof("Timeout reached: count=%d/%d", count.Load(), total)
					wg.Wait()
					finalCount := count.Load()
					if finalCount >= int32(total) {
						// All items actually completed before timeout
						t.Infof("Completed batch %s %d of %d", b.Name, finalCount, total)
						taskMu.Lock()
						t.Success()
						taskMu.Unlock()
						closeResults()
						done <- nil
					} else {
						// Genuinely timed out
						t.Errorf("Batch timeout: %s (completed %d of %d)", b.Name, finalCount, total)
						taskMu.Lock()
						err := fmt.Errorf("batch timeout after 5 hours")
						_, _ = t.FailedWithError(err)
						taskMu.Unlock()
						closeResults()
						done <- err
					}
					return
				case <-ticker.C:
					currentCount := count.Load()
					t.Debugf("Monitoring tick: count=%d/%d", currentCount, total)
					if currentCount >= int32(total) {
						// All items completed
						t.Infof("Completed batch %s %d of %d", b.Name, currentCount, total)
						wg.Wait()
						taskMu.Lock()
						t.Success()
						taskMu.Unlock()
						closeResults()
						done <- nil
						return
					}
					t.Infof("Waiting %d of %d", currentCount, total)
				}
			}
		}()

		// Wait for monitoring goroutine to complete and signal status
		t.Debugf("Waiting for monitoring goroutine to signal completion")
		err := <-done
		t.Debugf("Batch.Run finished: %s, error=%v", b.Name, err)
		return nil, err
	}).SetLogLevel(logger.Trace1)

	return results
}
