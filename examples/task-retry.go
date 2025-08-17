package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/spf13/pflag"
)

// Service represents an external service that might fail
type Service struct {
	name         string
	failureRate  float32
	attemptCount int
}

func (s *Service) Call(t *task.Task) error {
	s.attemptCount++
	t.Infof("Attempt #%d to connect to %s", s.attemptCount, s.name)

	// Simulate network delay
	time.Sleep(time.Duration(200+rand.Intn(300)) * time.Millisecond)

	// Random failure based on failure rate
	if rand.Float32() < s.failureRate {
		err := fmt.Errorf("connection to %s failed (attempt %d)", s.name, s.attemptCount)
		t.Errorf("Failed: %v", err)
		return err
	}

	t.Infof("Successfully connected to %s", s.name)
	return nil
}

func main() {
	// Setup flags
	flags := clicky.BindAllFlags(pflag.CommandLine)

	failureRate := pflag.Float32("failure-rate", 0.5, "Simulated failure rate (0.0-1.0)")

	pflag.Parse()

	flags.UseFlags()

	fmt.Println(clicky.MustFormat(clicky.Flags))
	// Create task manager with retry configuration
	tm := task.NewManagerWithOptions(&flags.TaskManagerOptions)

	fmt.Printf("=== Error Handling & Retry Example ===\n")
	fmt.Printf("Simulated Failure Rate: %.0f%%\n\n", *failureRate*100)

	// Create services with different reliability
	services := []*Service{
		{name: "Database", failureRate: *failureRate * 0.5},       // More reliable
		{name: "Cache", failureRate: *failureRate * 0.3},          // Most reliable
		{name: "External API", failureRate: *failureRate * 1.2},   // Less reliable
		{name: "Message Queue", failureRate: *failureRate},        // Average reliability
		{name: "Search Service", failureRate: *failureRate * 1.5}, // Least reliable
	}

	// Task 1: Retriable task with custom retry logic
	dbTask := tm.Start("Connect to Database",
		task.WithRetryConfig(task.DefaultRetryConfig()),
		task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
			return services[0].Call(t)
		}))

	// Task 2: Non-retriable critical task
	criticalTask := tm.Start("Critical Operation",
		// No retry config for non-retriable tasks
		task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
			t.Infof("Performing critical one-time operation")
			time.Sleep(500 * time.Millisecond)

			// 20% chance of failure
			if rand.Float32() < 0.2 {
				err := fmt.Errorf("critical operation failed - no retry allowed")
				t.Errorf("%v", err)
				return err
			}

			t.Success()
			return nil
		}))

	// Task 3: Task with custom retry handler
	apiTask := tm.Start("Call External API",
		task.WithRetryConfig(task.DefaultRetryConfig()),
		task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
			return services[2].Call(t)
		}))

	// Task 4: Task that succeeds after retries
	mqTask := tm.Start("Connect to Message Queue",
		task.WithRetryConfig(task.DefaultRetryConfig()),
		task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
			// This task gets more reliable with each attempt (simulating recovery)
			service := services[3]
			if service.attemptCount > 0 {
				service.failureRate *= 0.5 // Halve failure rate each retry
			}
			return service.Call(t)
		}))

	// Task 5: Task with timeout and retry
	searchTask := tm.Start("Initialize Search Service",
		task.WithRetryConfig(task.DefaultRetryConfig()),
		task.WithTimeout(2*time.Second), // Overall timeout
		task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
			select {
			case <-ctx.Done():
				return fmt.Errorf("task timeout exceeded")
			default:
				return services[4].Call(t)
			}
		}))

	// Task 6: Batch operation with partial failure handling
	batchTask := tm.Start("Process Batch",
		task.WithRetryConfig(task.DefaultRetryConfig()),
		task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
			totalItems := 10
			failedItems := 0

			t.Infof("Processing batch of %d items", totalItems)

			for i := 1; i <= totalItems; i++ {
				t.SetProgress(i, totalItems)

				// Random failure for each item
				if rand.Float32() < 0.3 {
					failedItems++
					t.Warnf("Item %d failed to process", i)
				} else {
					t.Debugf("Item %d processed successfully", i)
				}

				time.Sleep(100 * time.Millisecond)
			}

			// Fail if more than 30% of items failed
			failureThreshold := float32(failedItems) / float32(totalItems)
			if failureThreshold > 0.3 {
				err := fmt.Errorf("batch processing failed: %d/%d items failed (%.0f%%)",
					failedItems, totalItems, failureThreshold*100)
				t.Errorf("%v", err)
				return err
			}

			if failedItems > 0 {
				t.Warnf("Completed with %d failed items", failedItems)
				t.Warning()
			} else {
				t.Success()
			}

			return nil
		}))

	// Create error recovery workflow
	fmt.Println("\n--- Error Recovery Workflow ---")

	// Primary task that might fail
	primaryOp := tm.Start("Primary Operation",
		task.WithRetryConfig(task.DefaultRetryConfig()),
		task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
			t.Infof("Attempting primary operation")

			if rand.Float32() < 0.7 { // 70% failure rate
				return fmt.Errorf("primary operation failed")
			}

			t.Success()
			return nil
		}))

	// Fallback task that runs if primary fails
	fallbackOp := tm.Start("Fallback Operation",
		task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
			// Wait for primary to complete
			primaryOp.WaitFor()

			if primaryOp.Status() == task.StatusFailed {
				t.Infof("Primary failed, executing fallback")
				time.Sleep(500 * time.Millisecond)
				t.Success()
				t.Infof("Fallback completed successfully")
			} else {
				t.Infof("Primary succeeded, skipping fallback")
				// Mark as success since fallback not needed
				t.Success()
			}

			return nil
		}))

	// Wait for all tasks
	exitCode := tm.Wait()

	// Collect retry statistics
	fmt.Println("\n=== Retry Statistics ===")

	retryStats := struct {
		TotalTasks         int
		RetriedTasks       int
		SuccessfulRetries  int
		FailedAfterRetries int
		ServiceAttempts    map[string]int
	}{
		ServiceAttempts: make(map[string]int),
	}

	allTasks := []*task.Task{
		dbTask, criticalTask, apiTask, mqTask,
		searchTask, batchTask, primaryOp, fallbackOp,
	}

	retryStats.TotalTasks = len(allTasks)

	// Since RetryCount is not exported, track based on status
	for _, t := range allTasks {
		// Count tasks that eventually succeeded or warned as successful retries
		if t.Status() == task.StatusSuccess || t.Status() == task.StatusWarning {
			// Tasks that succeeded might have been retried
			retryStats.SuccessfulRetries++
		} else if t.Status() == task.StatusFailed {
			retryStats.FailedAfterRetries++
		}
	}

	// Service attempt counts
	for _, service := range services {
		if service.attemptCount > 0 {
			retryStats.ServiceAttempts[service.name] = service.attemptCount
		}
	}

	// Display statistics
	output, err := clicky.Format(retryStats)
	if err != nil {
		fmt.Printf("Total Tasks: %d\n", retryStats.TotalTasks)
		fmt.Printf("Tasks that retried: %d\n", retryStats.RetriedTasks)
		fmt.Printf("Successful after retry: %d\n", retryStats.SuccessfulRetries)
		fmt.Printf("Failed after all retries: %d\n", retryStats.FailedAfterRetries)
		fmt.Println("\nService Attempts:")
		for service, attempts := range retryStats.ServiceAttempts {
			fmt.Printf("  %s: %d attempts\n", service, attempts)
		}
	} else {
		fmt.Println(output)
	}

	// Task details
	fmt.Println("\n=== Task Results ===")
	for _, t := range allTasks {
		status := "✓"
		statusColor := "green"
		if t.Status() == task.StatusFailed {
			status = "✗"
			statusColor = "red"
		} else if t.Status() == task.StatusWarning {
			status = "⚠"
			statusColor = "yellow"
			// StatusSkipped doesn't exist, skip this case
			// } else if t.Status() == task.StatusSkipped {
			//	status = "⊘"
			//	statusColor = "gray"
		}

		retryInfo := ""
		// RetryCount is not exported, can't display retry count
		// if t.RetryCount() > 0 {
		//	retryInfo = fmt.Sprintf(" (retried %d times)", t.RetryCount())
		// }

		fmt.Printf("%s %s: %s%s\n", status, t.Name(), t.Status(), retryInfo)

		// Show error if failed
		if t.Status() == task.StatusFailed && t.Error() != nil {
			fmt.Printf("  └─ Error: %v\n", t.Error())
		}

		_ = statusColor // Color would be used in a real terminal UI
	}

	os.Exit(exitCode)
}
