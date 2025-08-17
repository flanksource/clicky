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

func main() {
	// Setup flags
	flags := clicky.BindAllFlags(pflag.CommandLine)

	// Custom flags for this example
	numTasks := pflag.Int("num-tasks", 10, "Number of concurrent tasks to create")
	maxWorkers := pflag.Int("max-workers", 3, "Maximum concurrent workers")
	taskDuration := pflag.Duration("task-duration", 2*time.Second, "Maximum duration for each task")

	pflag.Parse()

	// Override max concurrent from command line
	if *maxWorkers > 0 {
		flags.MaxConcurrent = *maxWorkers
	}

	flags.UseFlags()

	// Create task manager
	tm := task.Global

	fmt.Printf("=== Concurrent Tasks Example ===\n")
	fmt.Printf("Creating %d tasks with max %d concurrent workers\n\n", *numTasks, flags.MaxConcurrent)

	// Create multiple tasks with different priorities
	var tasks []*task.Task

	for i := 1; i <= *numTasks; i++ {
		taskName := fmt.Sprintf("Task-%02d", i)
		taskNum := i // Capture loop variable

		// Assign different priorities (lower number = higher priority)
		priority := 5 // Normal priority
		if taskNum <= 2 {
			priority = 1 // High priority (first 2 tasks)
		} else if taskNum > *numTasks-2 {
			priority = 10 // Low priority (last 2 tasks)
		}

		t := tm.Start(taskName,
			task.WithPriority(priority),
			task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
				// Random work duration
				workDuration := time.Duration(rand.Int63n(int64(*taskDuration)))

				t.Infof("Starting with priority %d, estimated duration: %v", priority, workDuration)

				// Simulate work with progress updates
				steps := 10
				stepDuration := workDuration / time.Duration(steps)

				for step := 1; step <= steps; step++ {
					select {
					case <-ctx.Done():
						t.Warnf("Task cancelled at step %d", step)
						return ctx.Err()
					case <-time.After(stepDuration):
						t.SetProgress(step, steps)

						// Occasionally log something
						if step%3 == 0 {
							t.Debugf("Processing step %d/%d", step, steps)
						}
					}
				}

				// Random chance of warning
				if rand.Float32() < 0.2 {
					t.Warnf("Completed with minor issues")
					t.Warning()
				} else {
					t.Success()
				}

				t.Infof("Completed in %v", workDuration)
				return nil
			}))

		tasks = append(tasks, t)

		// Small delay between task creation to show queueing
		time.Sleep(50 * time.Millisecond)
	}

	// Monitor running tasks periodically
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Count running tasks manually
				running := 0
				pending := 0
				for _, t := range tasks {
					if t.Status() == task.StatusRunning {
						running++
					} else if t.Status() == task.StatusPending {
						pending++
					}
				}
				if running > 0 || pending > 0 {
					fmt.Printf("[Monitor] Running tasks: %d, Pending tasks: %d\n", running, pending)
				}
			case <-time.After(time.Duration(*numTasks) * *taskDuration):
				return
			}
		}
	}()

	// Create a high-priority task after a delay
	go func() {
		time.Sleep(2 * time.Second)
		urgentTask := tm.Start("URGENT-Task",
			task.WithPriority(0), // Highest priority (0)
			task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
				t.Errorf("⚡ URGENT task executing - should run before lower priority tasks")
				time.Sleep(500 * time.Millisecond)
				t.Success()
				return nil
			}))
		tasks = append(tasks, urgentTask)
	}()

	// Wait for completion
	exitCode := tm.Wait()

	// Generate statistics
	stats := struct {
		TotalTasks      int
		Successful      int
		Warnings        int
		Failed          int
		MaxConcurrent   int
		AverageWaitTime string
	}{
		TotalTasks:    len(tasks),
		MaxConcurrent: flags.MaxConcurrent,
	}

	var totalWait time.Duration
	for _, t := range tasks {
		// Count statuses
		switch t.Status() {
		case task.StatusSuccess:
			stats.Successful++
		case task.StatusWarning:
			stats.Warnings++
		case task.StatusFailed:
			stats.Failed++
		}

		// Use duration instead of wait time calculation
		totalWait += t.Duration()
	}

	if len(tasks) > 0 {
		stats.AverageWaitTime = (totalWait / time.Duration(len(tasks))).String()
	}

	// Display results
	fmt.Printf("\n=== Execution Statistics ===\n")
	output, err := clicky.Format(stats)
	if err != nil {
		fmt.Printf("Total tasks: %d\n", stats.TotalTasks)
		fmt.Printf("Successful: %d\n", stats.Successful)
		fmt.Printf("Warnings: %d\n", stats.Warnings)
		fmt.Printf("Failed: %d\n", stats.Failed)
		fmt.Printf("Max concurrent: %d\n", stats.MaxConcurrent)
		fmt.Printf("Average wait time: %s\n", stats.AverageWaitTime)
	} else {
		fmt.Println(output)
	}

	// Priority distribution (track from our original setup)
	priorityCount := make(map[int]int)
	for i := range tasks {
		// Reconstruct priority based on original logic
		priority := 5 // Normal
		if i+1 <= 2 {
			priority = 1 // High
		} else if i+1 > *numTasks-2 {
			priority = 10 // Low
		}
		priorityCount[priority]++
	}

	fmt.Printf("\n=== Priority Distribution ===\n")
	for priority, count := range priorityCount {
		var label string
		switch priority {
		case 0:
			label = "Urgent"
		case 1:
			label = "High"
		case 5:
			label = "Normal"
		case 10:
			label = "Low"
		default:
			label = fmt.Sprintf("Priority-%d", priority)
		}
		fmt.Printf("%s: %d tasks\n", label, count)
	}

	os.Exit(exitCode)
}
