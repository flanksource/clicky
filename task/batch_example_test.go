package task_test

import (
	"errors"
	"fmt"
	"time"

	"github.com/flanksource/clicky/task"
	"github.com/flanksource/commons/logger"
)

// ExampleBatch_timeout demonstrates using a batch timeout to limit total execution time
func ExampleBatch_timeout() {
	batch := &task.Batch[string]{
		Name:       "example-batch-timeout",
		Timeout:    500 * time.Millisecond, // Batch must complete within 500ms
		MaxWorkers: 2,
	}

	// Add 10 items that each take 200ms - only ~5 will complete before timeout
	for i := 0; i < 10; i++ {
		i := i
		batch.Items = append(batch.Items, func(log logger.Logger) (string, error) {
			time.Sleep(200 * time.Millisecond)
			return fmt.Sprintf("item-%d", i), nil
		})
	}

	completed := []string{}
	timedOut := false

	for result := range batch.Run() {
		if result.Error != nil {
			if errors.Is(result.Error, task.ErrBatchTimeout) {
				timedOut = true
				fmt.Println("Batch timeout occurred")
			}
		} else {
			completed = append(completed, result.Value)
		}
	}

	fmt.Printf("Completed %d items before timeout: %v\n", len(completed), timedOut)
	// Output:
	// Batch timeout occurred
	// Completed 5 items before timeout: true
}

// ExampleBatch_itemTimeout demonstrates using per-item timeouts
func ExampleBatch_itemTimeout() {
	batch := &task.Batch[int]{
		Name:        "example-item-timeout",
		ItemTimeout: 100 * time.Millisecond, // Each item must complete within 100ms
		MaxWorkers:  3,
	}

	// Add 6 items: 3 fast (50ms), 3 slow (150ms)
	for i := 0; i < 6; i++ {
		i := i
		batch.Items = append(batch.Items, func(log logger.Logger) (int, error) {
			if i%2 == 0 {
				time.Sleep(50 * time.Millisecond)
			} else {
				time.Sleep(150 * time.Millisecond)
			}
			return i, nil
		})
	}

	completed := []int{}
	itemTimeouts := 0

	for result := range batch.Run() {
		if result.Error != nil {
			if errors.Is(result.Error, task.ErrItemTimeout) {
				itemTimeouts++
			}
		} else {
			completed = append(completed, result.Value)
		}
	}

	fmt.Printf("Completed: %d, Timed out: %d\n", len(completed), itemTimeouts)
	// Output:
	// Completed: 3, Timed out: 3
}

// ExampleBatch_WithTimeout demonstrates method chaining
func ExampleBatch_WithTimeout() {
	batch := (&task.Batch[string]{
		Name:       "example-chaining",
		MaxWorkers: 3,
	}).WithTimeout(1 * time.Second).WithItemTimeout(200 * time.Millisecond)

	fmt.Printf("Batch timeout: %v, Item timeout: %v\n", batch.Timeout, batch.ItemTimeout)
	// Output:
	// Batch timeout: 1s, Item timeout: 200ms
}

// ExampleBatch_zeroTimeout demonstrates zero-value backward compatibility
func ExampleBatch_zeroTimeout() {
	batch := &task.Batch[int]{
		Name:       "example-zero-timeout",
		Timeout:    0, // Zero means no timeout (backward compatible)
		MaxWorkers: 2,
	}

	// Add items that would take a while
	for i := 0; i < 5; i++ {
		i := i
		batch.Items = append(batch.Items, func(log logger.Logger) (int, error) {
			time.Sleep(50 * time.Millisecond)
			return i, nil
		})
	}

	count := 0
	for result := range batch.Run() {
		if result.Error == nil {
			count++
		}
	}

	fmt.Printf("All %d items completed (no timeout)\n", count)
	// Output:
	// All 5 items completed (no timeout)
}
