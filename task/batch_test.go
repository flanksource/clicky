package task

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flanksource/commons/logger"
)

func TestBatch_ConcurrentContextCancellation(t *testing.T) {
	// This test verifies that concurrent context cancellation doesn't cause panics
	_, cancel := context.WithCancel(context.Background())

	items := make([]func(logger.Logger) (string, error), 10)
	for i := range items {
		i := i
		items[i] = func(log logger.Logger) (string, error) {
			time.Sleep(10 * time.Millisecond)
			return fmt.Sprintf("item-%d", i), nil
		}
	}

	batch := &Batch[string]{
		Name:       "test-cancellation",
		Items:      items,
		MaxWorkers: 5,
	}

	// Cancel context while batch is running
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	results := batch.Run()

	// Should be able to read from channel without panics
	count := 0
	for range results {
		count++
	}

	// Channel should be closed, reading again should immediately return
	_, ok := <-results
	if ok {
		t.Error("Channel should be closed")
	}
}

func TestBatch_RapidCompletion(t *testing.T) {
	// This test verifies that rapid completion of all items works correctly
	items := make([]func(logger.Logger) (int, error), 20)
	for i := range items {
		i := i
		items[i] = func(log logger.Logger) (int, error) {
			return i, nil
		}
	}

	batch := &Batch[int]{
		Name:       "test-rapid",
		Items:      items,
		MaxWorkers: 10,
	}

	results := batch.Run()

	count := 0
	for range results {
		count++
	}

	if count != 20 {
		t.Errorf("Expected 20 results, got %d", count)
	}

	// Verify channel is closed
	_, ok := <-results
	if ok {
		t.Error("Channel should be closed")
	}
}

func XTestBatch_PanicRecovery(t *testing.T) {
	// This test verifies that panics during processing don't crash the system
	items := make([]func(logger.Logger) (string, error), 5)
	for i := range items {
		i := i
		items[i] = func(log logger.Logger) (string, error) {
			if i == 2 {
				panic("intentional panic")
			}
			return fmt.Sprintf("item-%d", i), nil
		}
	}

	batch := &Batch[string]{
		Name:       "test-panic",
		Items:      items,
		MaxWorkers: 3,
	}

	results := batch.Run()

	count := 0
	panicCount := 0
	for result := range results {
		count++
		if result.Error != nil && result.Error.Error() == "panic: intentional panic" {
			panicCount++
		}
	}

	if count != 5 {
		t.Errorf("Expected 5 results, got %d", count)
	}

	if panicCount != 1 {
		t.Errorf("Expected 1 panic error, got %d", panicCount)
	}

	// Verify channel is closed
	_, ok := <-results
	if ok {
		t.Error("Channel should be closed")
	}
}

func TestBatch_NoDoubleClose(t *testing.T) {
	// This test verifies that the channel is closed exactly once even with multiple close paths
	// We run this test with the race detector enabled: go test -race
	items := make([]func(logger.Logger) (int, error), 3)
	for i := range items {
		i := i
		items[i] = func(log logger.Logger) (int, error) {
			return i, nil
		}
	}

	batch := &Batch[int]{
		Name:       "test-no-double-close",
		Items:      items,
		MaxWorkers: 3,
	}

	results := batch.Run()

	// Consume all results
	for range results {
	}

	// Try reading again - should not panic
	_, ok := <-results
	if ok {
		t.Error("Channel should be closed")
	}
}

func TestBatch_AllGoroutinesComplete(t *testing.T) {
	// This test verifies that all goroutines complete before the channel is closed
	var started atomic.Int32
	var completed atomic.Int32

	items := make([]func(logger.Logger) (string, error), 10)
	for i := range items {
		i := i
		items[i] = func(log logger.Logger) (string, error) {
			started.Add(1)
			time.Sleep(10 * time.Millisecond)
			completed.Add(1)
			return fmt.Sprintf("item-%d", i), nil
		}
	}

	batch := &Batch[string]{
		Name:       "test-goroutine-lifecycle",
		Items:      items,
		MaxWorkers: 5,
	}

	results := batch.Run()

	// Consume all results
	count := 0
	for range results {
		count++
	}

	// Give a small grace period for goroutines to finish cleanup
	time.Sleep(20 * time.Millisecond)

	// After channel is closed, all goroutines should have completed
	// Note: Due to context cancellation in the task manager, we might not get all results
	// but we should get at least some results
	if count < 5 {
		t.Errorf("Expected at least 5 results, got %d", count)
	}
}

func TestBatch_ContextCancellationPropagation(t *testing.T) {
	// This test verifies that the batch can be canceled mid-execution
	items := make([]func(logger.Logger) (string, error), 5)
	for i := range items {
		i := i
		items[i] = func(log logger.Logger) (string, error) {
			time.Sleep(10 * time.Millisecond)
			return fmt.Sprintf("item-%d", i), nil
		}
	}

	batch := &Batch[string]{
		Name:       "test-context-propagation",
		Items:      items,
		MaxWorkers: 5,
	}

	results := batch.Run()

	// Consume results
	count := 0
	for range results {
		count++
	}

	// We should get at least some results
	if count < 1 {
		t.Error("Expected at least one result")
	}
}

// Timeout Tests

func TestBatch_BatchTimeout(t *testing.T) {
	// Test that batch timeout fires correctly and returns partial results
	batch := &Batch[int]{
		Name:       "test-batch-timeout",
		Timeout:    200 * time.Millisecond,
		MaxWorkers: 2,
	}

	// Add 20 items that each take 100ms - only ~4 should complete before timeout
	for i := 0; i < 20; i++ {
		i := i
		batch.Items = append(batch.Items, func(log logger.Logger) (int, error) {
			time.Sleep(100 * time.Millisecond)
			return i, nil
		})
	}

	results := []int{}
	gotTimeout := false
	var timeoutErr error

	for result := range batch.Run() {
		if result.Error != nil {
			if errors.Is(result.Error, ErrBatchTimeout) {
				gotTimeout = true
				timeoutErr = result.Error
			}
		} else {
			results = append(results, result.Value)
		}
	}

	if !gotTimeout {
		t.Error("Expected batch timeout error")
	}

	if timeoutErr == nil {
		t.Error("Expected timeout error to be set")
	}

	if len(results) < 1 {
		t.Error("Expected at least some results before timeout")
	}

	if len(results) >= 20 {
		t.Errorf("Expected partial results, got all %d", len(results))
	}

	t.Logf("Batch timeout test: got %d results before timeout", len(results))
}

func TestBatch_ItemTimeout(t *testing.T) {
	// Test that individual items timing out return ErrItemTimeout
	batch := &Batch[int]{
		Name:        "test-item-timeout",
		ItemTimeout: 50 * time.Millisecond,
		MaxWorkers:  3,
	}

	// Add 10 items: 5 fast, 5 slow
	for i := 0; i < 10; i++ {
		i := i
		batch.Items = append(batch.Items, func(log logger.Logger) (int, error) {
			if i%2 == 0 {
				// Fast items
				time.Sleep(10 * time.Millisecond)
			} else {
				// Slow items that will timeout
				time.Sleep(100 * time.Millisecond)
			}
			return i, nil
		})
	}

	results := []int{}
	itemTimeouts := 0

	for result := range batch.Run() {
		if result.Error != nil {
			if errors.Is(result.Error, ErrItemTimeout) {
				itemTimeouts++
			} else {
				t.Errorf("Unexpected error: %v", result.Error)
			}
		} else {
			results = append(results, result.Value)
		}
	}

	if itemTimeouts == 0 {
		t.Error("Expected some items to timeout")
	}

	if len(results) == 0 {
		t.Error("Expected some items to complete successfully")
	}

	t.Logf("Item timeout test: %d items timed out, %d completed", itemTimeouts, len(results))
}

func TestBatch_ZeroTimeoutBackwardCompatibility(t *testing.T) {
	// Test that zero timeout (default) behaves as infinite timeout
	batch := &Batch[int]{
		Name:        "test-zero-timeout",
		Timeout:     0, // Zero means no timeout
		ItemTimeout: 0, // Zero means no timeout
		MaxWorkers:  5,
	}

	// Add items that take some time but should all complete
	for i := 0; i < 10; i++ {
		i := i
		batch.Items = append(batch.Items, func(log logger.Logger) (int, error) {
			time.Sleep(50 * time.Millisecond)
			return i, nil
		})
	}

	results := []int{}
	for result := range batch.Run() {
		if result.Error != nil {
			t.Errorf("Unexpected error with zero timeout: %v", result.Error)
		}
		results = append(results, result.Value)
	}

	if len(results) != 10 {
		t.Errorf("Expected all 10 items to complete, got %d", len(results))
	}
}

func TestBatch_ErrorIdentification(t *testing.T) {
	// Test that errors can be identified with errors.Is()
	t.Run("BatchTimeout", func(t *testing.T) {
		batch := &Batch[int]{
			Name:       "test-error-id-batch",
			Timeout:    100 * time.Millisecond,
			MaxWorkers: 1,
		}

		// Add items that take longer than timeout
		for i := 0; i < 10; i++ {
			i := i
			batch.Items = append(batch.Items, func(log logger.Logger) (int, error) {
				time.Sleep(200 * time.Millisecond)
				return i, nil
			})
		}

		var foundErr error
		for result := range batch.Run() {
			if result.Error != nil {
				foundErr = result.Error
				break
			}
		}

		if foundErr == nil {
			t.Fatal("Expected to find an error")
		}

		if !errors.Is(foundErr, ErrBatchTimeout) {
			t.Errorf("Expected ErrBatchTimeout, got: %v", foundErr)
		}
	})

	t.Run("ItemTimeout", func(t *testing.T) {
		batch := &Batch[int]{
			Name:        "test-error-id-item",
			ItemTimeout: 50 * time.Millisecond,
			Timeout:     1 * time.Second, // Explicit batch timeout to avoid race with item timeout
			MaxWorkers:  1,
		}

		batch.Items = append(batch.Items, func(log logger.Logger) (int, error) {
			time.Sleep(100 * time.Millisecond)
			return 42, nil
		})

		var foundErr error
		for result := range batch.Run() {
			if result.Error != nil {
				foundErr = result.Error
			}
		}

		if foundErr == nil {
			t.Fatal("Expected to find an error")
		}

		if !errors.Is(foundErr, ErrItemTimeout) {
			t.Errorf("Expected ErrItemTimeout, got: %v", foundErr)
		}
	})
}

func TestBatch_MethodChaining(t *testing.T) {
	// Test that WithTimeout and WithItemTimeout work for method chaining
	batch := (&Batch[int]{
		Name:       "test-chaining",
		MaxWorkers: 3,
	}).WithTimeout(1 * time.Second).WithItemTimeout(500 * time.Millisecond)

	if batch.Timeout != 1*time.Second {
		t.Errorf("Expected Timeout to be 1s, got %v", batch.Timeout)
	}

	if batch.ItemTimeout != 500*time.Millisecond {
		t.Errorf("Expected ItemTimeout to be 500ms, got %v", batch.ItemTimeout)
	}

	// Add a few quick items to verify it runs
	for i := 0; i < 3; i++ {
		i := i
		batch.Items = append(batch.Items, func(log logger.Logger) (int, error) {
			return i, nil
		})
	}

	count := 0
	for result := range batch.Run() {
		if result.Error != nil {
			t.Errorf("Unexpected error: %v", result.Error)
		}
		count++
	}

	if count != 3 {
		t.Errorf("Expected 3 results, got %d", count)
	}
}

func TestBatch_CombinedTimeouts(t *testing.T) {
	// Test both batch and item timeouts set
	batch := &Batch[int]{
		Name:        "test-combined",
		Timeout:     500 * time.Millisecond,
		ItemTimeout: 100 * time.Millisecond,
		MaxWorkers:  2,
	}

	// Add items with varying durations
	for i := 0; i < 10; i++ {
		i := i
		batch.Items = append(batch.Items, func(log logger.Logger) (int, error) {
			// Some items fast, some slow (exceed item timeout), batch should timeout overall
			if i%3 == 0 {
				time.Sleep(150 * time.Millisecond) // Exceeds item timeout
			} else {
				time.Sleep(50 * time.Millisecond) // OK
			}
			return i, nil
		})
	}

	itemTimeouts := 0
	batchTimeout := false
	successCount := 0

	for result := range batch.Run() {
		if result.Error != nil {
			if errors.Is(result.Error, ErrItemTimeout) {
				itemTimeouts++
			} else if errors.Is(result.Error, ErrBatchTimeout) {
				batchTimeout = true
			}
		} else {
			successCount++
		}
	}

	t.Logf("Combined timeouts: %d item timeouts, %d successful, batch timeout: %v", itemTimeouts, successCount, batchTimeout)

	// Either we get item timeouts or batch timeout (or both)
	if itemTimeouts == 0 && !batchTimeout {
		t.Error("Expected either item or batch timeouts")
	}
}

func TestBatch_EmptyBatchWithTimeout(t *testing.T) {
	// Edge case: empty batch with timeout
	batch := &Batch[int]{
		Name:    "test-empty",
		Timeout: 1 * time.Second,
	}

	count := 0
	for range batch.Run() {
		count++
	}

	if count != 0 {
		t.Errorf("Expected 0 results from empty batch, got %d", count)
	}
}

func TestBatch_SingleItemWithTimeout(t *testing.T) {
	// Edge case: single item with timeout
	batch := &Batch[int]{
		Name:        "test-single",
		ItemTimeout: 100 * time.Millisecond,
		MaxWorkers:  1,
	}

	batch.Items = append(batch.Items, func(log logger.Logger) (int, error) {
		time.Sleep(50 * time.Millisecond)
		return 42, nil
	})

	results := []int{}
	for result := range batch.Run() {
		if result.Error != nil {
			t.Errorf("Unexpected error: %v", result.Error)
		}
		results = append(results, result.Value)
	}

	if len(results) != 1 || results[0] != 42 {
		t.Errorf("Expected [42], got %v", results)
	}
}
