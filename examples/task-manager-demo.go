//go:build ignore

/*
Task Manager Demo - Comprehensive demonstration of clicky task manager capabilities

This demo showcases the clicky task manager with various scenarios including:
- Basic tasks with progress tracking
- Error handling and failures
- Task groups with concurrency control
- Stress testing with verbose logging

Usage:

	Build the demo:
	  go build -o task-manager-demo examples/task-manager-demo.go

	Run different scenarios:
	  ./task-manager-demo --scenario basic --num-tasks 5
	  ./task-manager-demo --scenario errors --error-rate 0.3
	  ./task-manager-demo --scenario groups --max-concurrent 3
	  ./task-manager-demo --scenario stress --log-volume high
	  ./task-manager-demo --scenario all

	Use different output formats:
	  ./task-manager-demo --scenario basic --format json
	  ./task-manager-demo --scenario basic --format yaml
	  ./task-manager-demo --scenario basic --format html

	Control logging:
	  ./task-manager-demo --scenario basic --log-volume low
	  ./task-manager-demo --scenario basic --verbose
	  ./task-manager-demo --scenario basic --loglevel -v

	Disable progress display:
	  ./task-manager-demo --scenario all --no-progress

	Combine flags:
	  ./task-manager-demo --scenario all --num-tasks 5 --error-rate 0.1 --max-concurrent 2 --no-progress

Available Flags:

	All standard clicky.BindAllFlags() are automatically installed:
	  - Format flags: --format, --json, --yaml, --csv, --html, --pdf, --markdown
	  - Task manager flags: --max-concurrent, --no-progress, --max-retries, --retry-delay
	  - Logging flags: --log-level, --loglevel, --verbose, --json-logs

	Demo-specific flags:
	  --scenario: basic, errors, groups, stress, all (default: all)
	  --num-tasks: Number of tasks per scenario (default: 10)
	  --error-rate: Error rate for tasks 0.0-1.0 (default: 0.2)
	  --log-volume: low, medium, high (default: medium)
*/
package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/spf13/cobra"
)

type DemoOptions struct {
	Scenario  string
	NumTasks  int
	ErrorRate float64
	LogVolume string
}

var clickyFlags *clicky.AllFlags

func main() {
	var opts DemoOptions

	rootCmd := &cobra.Command{
		Use:   "task-manager-demo",
		Short: "Comprehensive task manager demonstration",
		Long: `Demonstrates the clicky task manager with various scenarios:
  - basic: Simple successful tasks
  - errors: Tasks with various error conditions
  - groups: Grouped tasks with concurrency
  - stress: Many tasks with verbose logging
  - all: All scenarios combined`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDemo(opts)
		},
	}

	// Install clicky default flags
	clickyFlags = clicky.BindAllFlags(rootCmd.PersistentFlags())

	// Custom demo flags
	rootCmd.Flags().StringVar(&opts.Scenario, "scenario", "all", "Scenario to run: basic, errors, groups, stress, all")
	rootCmd.Flags().IntVar(&opts.NumTasks, "num-tasks", 10, "Number of tasks per scenario")
	rootCmd.Flags().Float64Var(&opts.ErrorRate, "error-rate", 0.2, "Error rate for tasks (0.0-1.0)")
	rootCmd.Flags().StringVar(&opts.LogVolume, "log-volume", "medium", "Log volume: low, medium, high")

	// Apply flags before running
	cobra.OnInitialize(func() {
		clickyFlags.UseFlags()
	})

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runDemo(opts DemoOptions) error {
	fmt.Printf("=== Task Manager Demo ===\n")
	fmt.Printf("Scenario: %s\n", opts.Scenario)
	fmt.Printf("Tasks per scenario: %d\n", opts.NumTasks)
	fmt.Printf("Error rate: %.0f%%\n", opts.ErrorRate*100)
	fmt.Printf("Log volume: %s\n\n", opts.LogVolume)

	// Run selected scenarios
	switch strings.ToLower(opts.Scenario) {
	case "basic":
		runBasicScenario(opts)
	case "errors":
		runErrorScenario(opts)
	case "groups":
		runGroupScenario(opts)
	case "stress":
		runStressScenario(opts)
	case "all":
		runBasicScenario(opts)
		runErrorScenario(opts)
		runGroupScenario(opts)
		runStressScenario(opts)
	default:
		return fmt.Errorf("unknown scenario: %s", opts.Scenario)
	}

	// Wait for all tasks
	exitCode := task.Wait()

	// Display summary
	fmt.Printf("\n=== Demo Complete ===\n")
	fmt.Printf("Exit code: %d\n", exitCode)

	os.Exit(exitCode)
	return nil
}

// Basic scenario: simple successful tasks
func runBasicScenario(opts DemoOptions) {
	fmt.Println("\n--- Basic Scenario ---")

	for i := 1; i <= opts.NumTasks; i++ {
		taskNum := i
		task.StartTask(fmt.Sprintf("Basic Task %d", i), func(ctx flanksourceContext.Context, t *task.Task) (any, error) {
			t.Infof("Starting basic task")

			// Simulate work with progress
			steps := 5
			for step := 1; step <= steps; step++ {
				time.Sleep(100 * time.Millisecond)
				t.SetProgress(step, steps)
				if shouldLog(opts.LogVolume, "debug") {
					t.Debugf("Processing step %d/%d", step, steps)
				}
			}

			t.Infof("Task %d completed successfully", taskNum)
			t.Success()
			return nil, nil
		})

		time.Sleep(20 * time.Millisecond)
	}
}

// Error scenario: tasks with various error conditions
func runErrorScenario(opts DemoOptions) {
	fmt.Println("\n--- Error Scenario ---")

	// Task that fails immediately
	task.StartTask("Immediate Failure", func(ctx flanksourceContext.Context, t *task.Task) (any, error) {
		t.Errorf("This task fails immediately")
		return nil, fmt.Errorf("immediate failure")
	})

	// Task that fails after some work
	task.StartTask("Delayed Failure", func(ctx flanksourceContext.Context, t *task.Task) (any, error) {
		t.Infof("Starting work...")
		for i := 1; i <= 3; i++ {
			time.Sleep(150 * time.Millisecond)
			t.SetProgress(i, 5)
			t.Infof("Step %d completed", i)
		}
		t.Errorf("Encountered error during processing")
		return nil, fmt.Errorf("processing failed at step 3")
	})

	// Task with warnings
	task.StartTask("Task with Warnings", func(ctx flanksourceContext.Context, t *task.Task) (any, error) {
		t.Infof("Processing data...")
		time.Sleep(200 * time.Millisecond)
		t.Warnf("Found 3 deprecated API calls")
		t.Warnf("Memory usage is high (85%%)")
		t.Warning()
		return nil, nil
	})

	// Tasks with random errors
	for i := 1; i <= opts.NumTasks; i++ {
		taskNum := i
		task.StartTask(fmt.Sprintf("Random Error Task %d", i), func(ctx flanksourceContext.Context, t *task.Task) (any, error) {
			t.Infof("Starting task")

			// Simulate work
			steps := 3
			for step := 1; step <= steps; step++ {
				time.Sleep(100 * time.Millisecond)
				t.SetProgress(step, steps)

				// Random failure
				if rand.Float64() < opts.ErrorRate {
					t.Errorf("Random error occurred at step %d", step)
					return nil, fmt.Errorf("task %d failed at step %d", taskNum, step)
				}

				if shouldLog(opts.LogVolume, "info") {
					t.Infof("Step %d completed", step)
				}
			}

			t.Success()
			return nil, nil
		})
	}
}

// Group scenario: tasks organized in groups
func runGroupScenario(opts DemoOptions) {
	fmt.Println("\n--- Group Scenario ---")

	// Create groups with different concurrency settings
	group1 := task.StartGroup[any]("Download Group", task.WithConcurrency(2))
	for i := 1; i <= 5; i++ {
		taskNum := i
		group1.Add(fmt.Sprintf("Download %d", i), func(ctx flanksourceContext.Context, t *task.Task) (any, error) {
			t.Infof("Downloading file %d", taskNum)
			time.Sleep(time.Duration(200+rand.Intn(300)) * time.Millisecond)

			if shouldLog(opts.LogVolume, "info") {
				t.Infof("Downloaded %d KB", 100+rand.Intn(900))
			}

			t.Success()
			return nil, nil
		})
	}

	group2 := task.StartGroup[any]("Process Group", task.WithConcurrency(3))
	for i := 1; i <= 5; i++ {
		taskNum := i
		group2.Add(fmt.Sprintf("Process %d", i), func(ctx flanksourceContext.Context, t *task.Task) (any, error) {
			t.Infof("Processing item %d", taskNum)

			steps := 4
			for step := 1; step <= steps; step++ {
				time.Sleep(100 * time.Millisecond)
				t.SetProgress(step, steps)

				if shouldLog(opts.LogVolume, "debug") {
					t.Debugf("Processing phase %d", step)
				}
			}

			t.Success()
			return nil, nil
		})
	}

	// Group with mixed results
	group3 := task.StartGroup[any]("Validation Group", task.WithConcurrency(2))
	for i := 1; i <= 4; i++ {
		taskNum := i
		group3.Add(fmt.Sprintf("Validate %d", i), func(ctx flanksourceContext.Context, t *task.Task) (any, error) {
			t.Infof("Validating item %d", taskNum)
			time.Sleep(150 * time.Millisecond)

			// Mix of success, warning, and error
			switch taskNum % 3 {
			case 0:
				t.Warnf("Validation passed with warnings")
				t.Warning()
			case 1:
				if rand.Float64() < opts.ErrorRate {
					t.Errorf("Validation failed")
					return nil, fmt.Errorf("validation error")
				}
				t.Success()
			default:
				t.Success()
			}

			return nil, nil
		})
	}
}

// Stress scenario: many tasks with verbose logging
func runStressScenario(opts DemoOptions) {
	fmt.Println("\n--- Stress Scenario ---")

	numStressTasks := opts.NumTasks * 2

	for i := 1; i <= numStressTasks; i++ {
		taskNum := i
		task.StartTask(fmt.Sprintf("Stress Task %d", i), func(ctx flanksourceContext.Context, t *task.Task) (any, error) {
			t.Infof("Starting stress test task %d", taskNum)

			// Generate many logs based on volume setting
			logCount := getLogCount(opts.LogVolume)

			for logNum := 1; logNum <= logCount; logNum++ {
				// Vary log levels
				switch logNum % 5 {
				case 0:
					t.Errorf("Error log %d: simulated error condition", logNum)
				case 1:
					t.Warnf("Warning log %d: potential issue detected", logNum)
				case 2:
					t.Infof("Info log %d: processing data batch", logNum)
				default:
					t.Debugf("Debug log %d: detailed trace information", logNum)
				}

				// Update progress
				t.SetProgress(logNum, logCount)

				// Small delay
				time.Sleep(10 * time.Millisecond)
			}

			// Random outcome
			randVal := rand.Float64()
			if randVal < opts.ErrorRate {
				t.Errorf("Stress task failed after %d logs", logCount)
				return nil, fmt.Errorf("stress test failure")
			} else if randVal < opts.ErrorRate*2 {
				t.Warnf("Stress task completed with warnings")
				t.Warning()
			} else {
				t.Infof("Stress task completed successfully")
				t.Success()
			}

			return nil, nil
		})

		// Minimal delay to create stress
		time.Sleep(5 * time.Millisecond)
	}
}

// Helper functions

func shouldLog(volume, level string) bool {
	switch strings.ToLower(volume) {
	case "low":
		return level == "error"
	case "medium":
		return level == "error" || level == "warn" || level == "info"
	case "high":
		return true
	default:
		return level == "error" || level == "warn" || level == "info"
	}
}

func getLogCount(volume string) int {
	switch strings.ToLower(volume) {
	case "low":
		return 5
	case "medium":
		return 20
	case "high":
		return 50
	default:
		return 20
	}
}
