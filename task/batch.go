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
		sem := semaphore.NewWeighted(int64(b.MaxWorkers))
		count := atomic.Int32{}
		t.SetName(fmt.Sprintf("%s %d of %d (concurrency:%d)", b.Name, 0, total, b.MaxWorkers))
		t.SetProgress(0, total)

		for i, item := range b.Items {
			logger.Infof("Queuing %v %d of %d", item, i+1, total)

			// Check for context cancellation before acquiring semaphore
			if ctx.Err() != nil {
				closeResults()
				return nil, ctx.Err()
			}

			if err := sem.Acquire(ctx, 1); err != nil {
				logger.Errorf("failed to acquire semaphore: %v", err)
				closeResults()
				return nil, err
			}
			logger.Infof("Acquired semaphore %v %d of %d", item, i+1, total)

			wg.Add(1)
			go func(item func(log logger.Logger) (T, error), itemNum int) {
				defer sem.Release(1)
				defer wg.Done()

				// Panic recovery to prevent goroutine crashes
				defer func() {
					if r := recover(); r != nil {
						logger.Errorf("panic in batch item %d: %v", itemNum, r)
						results <- BatchResult[T]{Error: fmt.Errorf("panic: %v", r)}
					}
				}()

				// Check for context cancellation before executing
				if ctx.Err() != nil {
					results <- BatchResult[T]{Error: ctx.Err()}
					return
				}

				start := time.Now()
				logger.Infof("Running %v %d of %d", item, itemNum, total)

				value, err := item(t)
				duration := time.Since(start)
				results <- BatchResult[T]{Value: value, Error: err, Duration: duration}
				newCount := count.Add(1)

				// Protect concurrent task updates with mutex
				taskMu.Lock()
				t.SetName(fmt.Sprintf("%s %d of %d", b.Name, newCount, total))
				t.SetProgress(int(newCount), total)
				taskMu.Unlock()

				logger.Infof("Finished %v %d of %d", item, newCount, total)
			}(item, i+1)
		}

		// Monitoring goroutine with timeout
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			timeout := time.After(5 * time.Hour) // 5 hour timeout for batch completion

			for {
				select {
				case <-ctx.Done():
					// Task cancelled, wait for goroutines to finish then close
					logger.Infof("Batch cancelled: %s", b.Name)
					wg.Wait()
					closeResults()
					taskMu.Lock()
					t.SetStatus(StatusCancelled)
					taskMu.Unlock()
					return
				case <-timeout:
					// Timeout reached
					logger.Errorf("Batch timeout: %s (completed %d of %d)", b.Name, count.Load(), total)
					wg.Wait()
					taskMu.Lock()
					_, _ = t.FailedWithError(fmt.Errorf("batch timeout after 5 hours"))
					taskMu.Unlock()
					closeResults()
					return
				case <-ticker.C:
					if count.Load() >= int32(total) {
						// All items completed
						logger.Infof("Completed batch %s %d of %d", b.Name, count.Load(), total)
						wg.Wait()
						taskMu.Lock()
						t.Success()
						taskMu.Unlock()
						closeResults()
						return
					}
					logger.Infof("Waiting %d of %d", count.Load(), total)
				}
			}
		}()

		// Return immediately but goroutines continue in background
		return nil, nil
	}).SetLogLevel(logger.Trace1)

	return results
}
