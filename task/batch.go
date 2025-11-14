package task

import (
	"fmt"
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
	StartTask(b.Name, func(ctx flanksourceContext.Context, t *Task) (interface{}, error) {

		sem := semaphore.NewWeighted(int64(b.MaxWorkers))
		count := atomic.Int32{}
		t.SetName(fmt.Sprintf("%s %d of %d (concurrency:%d)", b.Name, 0, total, b.MaxWorkers))
		t.SetProgress(0, total)

		for i, item := range b.Items {
			logger.Infof("Queuing %v %d of %d", item, i+1, total)

			// Check for context cancellation before acquiring semaphore
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			if err := sem.Acquire(ctx, 1); err != nil {
				logger.Errorf("failed to acquire semaphore: %v", err)
				return nil, err
			}
			logger.Infof("Acquired semaphore %v %d of %d", item, i+1, total)

			go func(item func(log logger.Logger) (T, error), itemNum int) {
				defer sem.Release(1)

				// Panic recovery to prevent goroutine crashes
				defer func() {
					if r := recover(); r != nil {
						logger.Errorf("panic in batch item %d: %v", itemNum, r)
						count.Add(1)
						// Send error result so monitoring goroutine knows this item completed
						select {
						case results <- BatchResult[T]{Error: fmt.Errorf("panic: %v", r)}:
						case <-ctx.Done():
							// Context cancelled, don't block
						}
					}
				}()

				// Check for context cancellation before executing
				if ctx.Err() != nil {
					count.Add(1)
					select {
					case results <- BatchResult[T]{Error: ctx.Err()}:
					case <-ctx.Done():
						// Context cancelled, don't block
					}
					return
				}

				start := time.Now()
				logger.Infof("Running %v %d of %d", item, itemNum, total)

				value, err := item(t)
				duration := time.Since(start)
				newCount := count.Add(1)
				t.SetName(fmt.Sprintf("%s %d of %d", b.Name, newCount, total))
				t.SetProgress(int(newCount), total)

				logger.Infof("Finished %v %d of %d", item, newCount, total)

				// Send result with cancellation check
				select {
				case results <- BatchResult[T]{Value: value, Error: err, Duration: duration}:
				case <-ctx.Done():
					// Context cancelled, don't block on send
				}
			}(item, i+1)
		}

		// Monitoring goroutine with timeout
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			timeout := time.After(5 * time.Minute) // 5 minute timeout for batch completion

			for {
				select {
				case <-ctx.Done():
					// Task cancelled, close results and exit
					logger.Infof("Batch cancelled: %s", b.Name)
					close(results)
					t.SetStatus(StatusCancelled)
					return
				case <-timeout:
					// Timeout reached
					logger.Errorf("Batch timeout: %s (completed %d of %d)", b.Name, count.Load(), total)
					close(results)
					_, _ = t.FailedWithError(fmt.Errorf("batch timeout after 5 minutes"))
					return
				case <-ticker.C:
					if count.Load() >= int32(total) {
						// All items completed
						logger.Infof("Completed batch %s %d of %d", b.Name, count.Load(), total)
						close(results)
						t.Success()
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
