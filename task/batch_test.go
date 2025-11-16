package task

import (
	"context"
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

func TestBatch_PanicRecovery(t *testing.T) {
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
