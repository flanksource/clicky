package task

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/stretchr/testify/assert"
)

func TestGroupConcurrency(t *testing.T) {
	// Create a group with concurrency limit of 2
	group := StartGroup[int]("Concurrent Test", WithConcurrency(2))

	var activeCount int64
	var maxActiveCount int64
	var mu sync.Mutex
	var results []int

	// Add 5 tasks that each take 100ms
	for i := 0; i < 5; i++ {
		taskID := i + 1
		group.Add(
			"Task "+string(rune('A'+i)),
			func(ctx flanksourceContext.Context, t *Task) (int, error) {
				// Track active tasks
				active := atomic.AddInt64(&activeCount, 1)
				
				// Update max active count if needed
				for {
					current := atomic.LoadInt64(&maxActiveCount)
					if active <= current || atomic.CompareAndSwapInt64(&maxActiveCount, current, active) {
						break
					}
				}

				// Simulate work
				time.Sleep(100 * time.Millisecond)
				
				// Store result
				mu.Lock()
				results = append(results, taskID)
				mu.Unlock()

				// Decrement active count
				atomic.AddInt64(&activeCount, -1)
				
				return taskID, nil
			},
		)
	}

	// Wait for all tasks to complete
	group.WaitFor()

	// Verify concurrency was limited to 2
	assert.LessOrEqual(t, maxActiveCount, int64(2), "Expected max active tasks to be <= 2")
	assert.Equal(t, int64(0), activeCount, "Expected all tasks to complete")
	assert.Equal(t, 5, len(results), "Expected all 5 tasks to complete")
}

func TestGroupNoConcurrencyLimit(t *testing.T) {
	// Create a group without concurrency limit (default behavior)
	group := StartGroup[int]("No Limit Test")

	var activeCount int64
	var maxActiveCount int64
	var mu sync.Mutex
	var results []int

	// Add 3 tasks
	for i := 0; i < 3; i++ {
		taskID := i + 1
		group.Add(
			"Task "+string(rune('A'+i)),
			func(ctx flanksourceContext.Context, t *Task) (int, error) {
				// Track active tasks
				active := atomic.AddInt64(&activeCount, 1)
				
				// Update max active count
				for {
					current := atomic.LoadInt64(&maxActiveCount)
					if active <= current || atomic.CompareAndSwapInt64(&maxActiveCount, current, active) {
						break
					}
				}

				// Simulate brief work
				time.Sleep(50 * time.Millisecond)
				
				// Store result
				mu.Lock()
				results = append(results, taskID)
				mu.Unlock()

				// Decrement active count
				atomic.AddInt64(&activeCount, -1)
				
				return taskID, nil
			},
		)
	}

	// Wait for all tasks to complete
	group.WaitFor()

	// Without group-level concurrency limit, tasks run based on global manager limits
	// The important thing is that all tasks complete successfully
	assert.GreaterOrEqual(t, maxActiveCount, int64(1), "Expected at least 1 task to run")
	assert.Equal(t, int64(0), activeCount, "Expected all tasks to complete")
	assert.Equal(t, 3, len(results), "Expected all 3 tasks to complete")
}

func TestGroupConcurrencyZero(t *testing.T) {
	// Create a group with concurrency 0 (should behave like no limit)
	group := StartGroup[int]("Zero Limit Test", WithConcurrency(0))

	var activeCount int64
	var maxActiveCount int64

	// Add 3 tasks
	for i := 0; i < 3; i++ {
		taskID := i + 1
		group.Add(
			"Task "+string(rune('A'+i)),
			func(ctx flanksourceContext.Context, t *Task) (int, error) {
				// Track active tasks
				active := atomic.AddInt64(&activeCount, 1)
				
				// Update max active count
				for {
					current := atomic.LoadInt64(&maxActiveCount)
					if active <= current || atomic.CompareAndSwapInt64(&maxActiveCount, current, active) {
						break
					}
				}

				// Simulate brief work
				time.Sleep(50 * time.Millisecond)

				// Decrement active count
				atomic.AddInt64(&activeCount, -1)
				
				return taskID, nil
			},
		)
	}

	// Wait for all tasks to complete
	group.WaitFor()

	// With concurrency 0, tasks run based on global manager limits (no group-level semaphore)
	assert.GreaterOrEqual(t, maxActiveCount, int64(1), "Expected at least 1 task to run")
	assert.Equal(t, int64(0), activeCount, "Expected all tasks to complete")
}