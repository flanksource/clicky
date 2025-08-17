package main

import (
	"fmt"
	"os"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/spf13/pflag"
)

func main() {
	// Setup flags using clicky.AllFlags
	flags := clicky.BindAllFlags(pflag.CommandLine)

	// Add custom flags for this example
	simulateWork := pflag.Bool("simulate-work", true, "Simulate work with delays")
	simulateError := pflag.Bool("simulate-error", false, "Simulate an error in task 3")

	pflag.Parse()

	// Apply all flags (logging, formatting, task manager options)
	flags.UseFlags()

	// Create task manager with the configured options
	tm := task.NewManagerWithOptions(&flags.TaskManagerOptions)

	// Example 1: Simple task that succeeds
	task1 := tm.Start("Download dependencies", task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
		t.Infof("Starting dependency download")

		// Simulate progress
		for i := 1; i <= 5; i++ {
			if *simulateWork {
				time.Sleep(200 * time.Millisecond)
			}
			t.SetProgress(i, 5)
			t.Infof("Downloaded package %d of 5", i)
		}

		t.Success()
		return nil
	}))

	// Example 2: Task with warning
	task2 := tm.Start("Build project", task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
		t.Infof("Compiling source files")

		// Simulate build steps
		steps := []string{"Parsing", "Type checking", "Compiling", "Linking"}
		for i, step := range steps {
			if *simulateWork {
				time.Sleep(300 * time.Millisecond)
			}
			t.SetProgress(i+1, len(steps))
			t.Infof("%s...", step)
		}

		// Simulate a warning
		t.Warnf("Found 3 deprecated API calls")
		t.Warning()
		return nil
	}))

	// Example 3: Task that might fail
	task3 := tm.Start("Run tests", task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
		t.Infof("Running test suite")

		totalTests := 10
		for i := 1; i <= totalTests; i++ {
			if *simulateWork {
				time.Sleep(100 * time.Millisecond)
			}

			// Simulate error on test 7 if flag is set
			if *simulateError && i == 7 {
				t.Errorf("Test %d failed: assertion error", i)
				t.Failed()
				return fmt.Errorf("test %d failed", i)
			}

			t.SetProgress(i, totalTests)
			t.Infof("Test %d passed", i)
		}

		t.Success()
		return nil
	}))

	// Example 4: Task with custom status updates
	task4 := tm.Start("Deploy application", task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
		stages := []struct {
			name     string
			duration time.Duration
		}{
			{"Preparing environment", 500 * time.Millisecond},
			{"Uploading artifacts", 700 * time.Millisecond},
			{"Starting services", 400 * time.Millisecond},
			{"Health check", 300 * time.Millisecond},
		}

		for i, stage := range stages {
			t.SetStatus(task.StatusRunning)
			t.Infof("Stage: %s", stage.name)

			if *simulateWork {
				time.Sleep(stage.duration)
			}

			t.SetProgress(i+1, len(stages))
		}

		t.Success()
		t.Infof("Deployment completed successfully")
		return nil
	}))

	// Example 5: Quick task
	quickTask := tm.Start("Cleanup", task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
		t.Infof("Cleaning temporary files")
		if *simulateWork {
			time.Sleep(200 * time.Millisecond)
		}
		t.Success()
		return nil
	}))

	// Display information about tasks
	fmt.Println("\n=== Task Manager Basic Example ===")
	fmt.Printf("Created %d tasks\n", 5)
	fmt.Printf("Max concurrent: %d\n", flags.MaxConcurrent)
	fmt.Printf("Progress display: %v\n", !flags.NoProgress)
	fmt.Printf("Output format: %s\n\n", flags.Format)

	// Wait for all tasks to complete
	exitCode := tm.Wait()

	// Display results based on format
	if flags.Format == "json" || flags.Format == "yaml" {
		// Use clicky.Format for structured output
		output, err := clicky.Format(struct {
			Tasks []struct {
				Name   string
				Status string
			}
			ExitCode int
		}{
			Tasks: []struct {
				Name   string
				Status string
			}{
				{Name: task1.Pretty().ANSI(), Status: string(task1.Status())},
				{Name: task2.Pretty().ANSI(), Status: string(task2.Status())},
				{Name: task3.Pretty().ANSI(), Status: string(task3.Status())},
				{Name: task4.Pretty().ANSI(), Status: string(task4.Status())},
				{Name: quickTask.Pretty().ANSI(), Status: string(quickTask.Status())},
			},
			ExitCode: exitCode,
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to format output: %v\n", err)
		} else {
			fmt.Println(output)
		}
	} else {
		// For pretty format, the task manager already displayed progress
		fmt.Printf("\n=== Summary ===\n")
		fmt.Printf("All tasks completed with exit code: %d\n", exitCode)

		// Show task statuses
		allTasks := []*task.Task{task1, task2, task3, task4, quickTask}
		for _, t := range allTasks {
			status := t.Status()
			symbol := "✓"
			if status == task.StatusFailed {
				symbol = "✗"
			} else if status == task.StatusWarning {
				symbol = "⚠"
			}
			fmt.Printf("%s %s: %s\n", symbol, t.Name(), status)
		}
	}

	os.Exit(exitCode)
}
