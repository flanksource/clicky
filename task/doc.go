/*
Package task provides a comprehensive concurrent task execution system with progress tracking,
status management, and visual rendering capabilities.

# Quick Start

The task system provides a simple interface for running concurrent operations with visual feedback:

	import "github.com/flanksource/clicky/task"

	// Start a simple task
	task := task.StartTask("Download File", func(ctx flanksourceContext.Context, t *task.Task) error {
		t.Infof("Starting download...")
		// Perform work
		time.Sleep(2 * time.Second)
		t.Infof("Download complete")
		return nil
	})

	// Wait for completion
	result := task.WaitFor()
	if result.Error != nil {
		fmt.Printf("Task failed: %v\n", result.Error)
	}

# Core Concepts

# Task Lifecycle

Tasks progress through well-defined states with visual indicators:

- StatusPending (⏳): Task is queued but not yet started
- StatusRunning (⟳): Task is currently executing
- StatusSuccess (✓): Task completed successfully
- StatusFailed (✗): Task failed with an error
- StatusWarning (⚠): Task completed with warnings
- StatusCancelled (⊘): Task was canceled

# Manager

The Manager coordinates task execution and provides visual rendering:

	manager := task.NewManager(
		task.WithMaxConcurrency(5),        // Limit concurrent tasks
		task.WithVerbose(true),            // Enable verbose logging
		task.WithNoProgress(false),        // Show progress bars
	)

	// Tasks automatically use the global manager if none specified

# Task Groups

Groups organize related tasks and provide concurrency control:

	group := task.NewGroup("Database Migration",
		task.WithConcurrency(2),  // Max 2 concurrent tasks in group
	)

	// Add tasks to the group
	task1 := group.Add("Migrate Users", func(ctx, t) error { ... })
	task2 := group.Add("Migrate Orders", func(ctx, t) error { ... })

	// Wait for all tasks in the group
	result := group.WaitFor()

# TypedTask for Type Safety

TypedTask provides compile-time type safety for task results:

	// Define a typed task that returns a string
	task := task.StartTask("Fetch Data", func(ctx flanksourceContext.Context, t *task.Task) (string, error) {
		// Perform work and return typed result
		return "Hello, World!", nil
	})

	// Get typed result
	result, err := task.GetResult()
	// result is of type string

	// Or wait and get result in one call
	wait := task.WaitFor()
	if wait.Error != nil {
		// Handle error
	}

# TypedGroup

Groups can also be typed for consistent result handling:

	group := task.NewTypedGroup[UserData]("Load Users")

	user1 := group.Add("Load User 1", func(ctx, t) (UserData, error) {
		return loadUser(1)
	})

	user2 := group.Add("Load User 2", func(ctx, t) (UserData, error) {
		return loadUser(2)
	})

	// Get all results as map
	results, err := group.GetResults()
	if err != nil {
		return err
	}

	for task, userData := range results {
		fmt.Printf("Task %s loaded: %+v\n", task.Name(), userData)
	}

# Status and Health System

The task system includes a rich status and health reporting system:

# Status Types

Tasks support multiple status paradigms:

	// Standard task statuses
	task.SetStatus(task.StatusSuccess)
	task.SetStatus(task.StatusFailed)
	task.SetStatus(task.StatusWarning)

	// Test-style statuses
	task.SetStatus(task.StatusPASS)
	task.SetStatus(task.StatusFAIL)
	task.SetStatus(task.StatusSKIP)

# Health Mixin

Results can implement HealthMixin for automatic status determination:

	type DatabaseResult struct {
		Connected bool
		Error     error
	}

	func (r DatabaseResult) Health() task.Health {
		if r.Error != nil {
			return task.HealthError
		}
		if !r.Connected {
			return task.HealthWarning
		}
		return task.HealthOK
	}

	// Task status will automatically reflect the health
	task.SetResult(DatabaseResult{Connected: true})
	// Task status becomes StatusSuccess automatically

# Visual Styling

Status information includes visual styling with Tailwind CSS classes:

	status := task.StatusSuccess
	fmt.Println(status.Icon())  // ✓
	fmt.Println(status.Style()) // "text-green-600"

	// Apply status styling to text
	text := api.Text{Content: "Operation Complete"}
	styledText := status.Apply(text)

# Concurrency Control

# Manager-Level Concurrency

Control global task concurrency:

	manager := task.NewManager(task.WithMaxConcurrency(10))
	// Maximum 10 tasks running simultaneously

# Group-Level Concurrency

Fine-grained control within groups:

	group := task.NewGroup("API Calls", task.WithConcurrency(3))
	// Maximum 3 concurrent tasks within this group

# Semaphore-Based Control

Groups use semaphores for precise concurrency management:

	// Group automatically handles semaphore acquisition/release
	group.Add("Task 1", taskFunc1)  // Acquires semaphore
	group.Add("Task 2", taskFunc2)  // Waits if limit reached
	// Semaphore released when task completes

# Error Handling and Retry

# Basic Error Handling

Tasks provide multiple ways to handle errors:

	task := task.StartTask("Risky Operation", func(ctx, t) error {
		if someCondition {
			return errors.New("operation failed")
		}
		return nil
	})

	result := task.WaitFor()
	if result.Error != nil {
		fmt.Printf("Task failed: %v\n", result.Error)
		fmt.Printf("Status: %s\n", result.Status)
	}

# Retry Configuration

Tasks support automatic retry with exponential backoff:

	retryConfig := task.RetryConfig{
		RetryableErrors: []string{"timeout", "connection", "rate limit"},
		BaseDelay:      1 * time.Second,
		MaxDelay:       30 * time.Second,
		BackoffFactor:  2.0,
		JitterFactor:   0.1,
		MaxRetries:     3,
	}

	task := task.StartTaskWithOptions("Flaky Operation", taskFunc,
		task.WithRetry(retryConfig),
	)

# Fatal Errors

For unrecoverable errors that should stop execution:

	task.Fatal(errors.New("critical system failure"))
	// Immediately stops execution and exits program

# Logging and Progress

# Built-in Logging

Tasks include comprehensive logging capabilities:

	task := task.StartTask("Process Data", func(ctx, t) error {
		t.Infof("Starting data processing...")
		t.Debugf("Processing batch %d", batchNum)

		if err := processData(); err != nil {
			t.Errorf("Failed to process batch: %v", err)
			return err
		}

		t.Infof("Processing complete")
		return nil
	})

Log levels:
- t.Tracef(): Detailed tracing (lowest level)
- t.Debugf(): Debug information
- t.Infof(): General information
- t.Warnf(): Warnings
- t.Errorf(): Errors
- t.Fatalf(): Fatal errors

# Progress Tracking

Tasks support progress indication:

	task := task.StartTask("Upload Files", func(ctx, t) error {
		files := getFilesToUpload()
		total := len(files)

		for i, file := range files {
			t.SetProgress(i, total)
			err := uploadFile(file)
			if err != nil {
				return err
			}
		}

		t.SetProgress(total, total) // 100% complete
		return nil
	})

# Integration Examples

# CLI Tool Integration

	func runCommand(args []string) error {
		manager := task.NewManager(
			task.WithVerbose(verbose),
			task.WithNoProgress(noProgress),
		)

		// Start background tasks
		task1 := task.StartTask("Validate Input", validateInput)
		task2 := task.StartTask("Load Configuration", loadConfig)

		// Wait for prerequisites
		task1.WaitFor()
		task2.WaitFor()

		// Main processing
		mainTask := task.StartTask("Process Data", processData)
		return mainTask.WaitFor().Error
	}

# HTTP Server Integration

	func handleRequest(w http.ResponseWriter, r *http.Request) {
		requestID := generateRequestID()

		task := task.StartTask(fmt.Sprintf("Handle Request %s", requestID),
			func(ctx, t) error {
				t.Infof("Processing request from %s", r.RemoteAddr)

				// Process request
				result, err := processRequest(r)
				if err != nil {
					t.Errorf("Request failed: %v", err)
					return err
				}

				t.Infof("Request completed successfully")
				return writeResponse(w, result)
			})

		// Wait for completion
		if result := task.WaitFor(); result.Error != nil {
			http.Error(w, result.Error.Error(), 500)
		}
	}

# Batch Processing

	func processBatch(items []Item) error {
		group := task.NewTypedGroup[ProcessedItem]("Batch Processing",
			task.WithConcurrency(5),
		)

		// Process items concurrently
		for i, item := range items {
			group.Add(fmt.Sprintf("Process Item %d", i+1),
				func(ctx, t) (ProcessedItem, error) {
					return processItem(item)
				})
		}

		// Wait for all processing to complete
		results, err := group.GetResults()
		if err != nil {
			return fmt.Errorf("batch processing failed: %w", err)
		}

		// Handle results
		for task, result := range results {
			fmt.Printf("Task %s: %+v\n", task.Name(), result)
		}

		return nil
	}

# Testing Integration

The task system automatically detects test environments and adjusts behavior:

	func TestDataProcessing(t *testing.T) {
		// Progress bars and colors automatically disabled in tests
		task := task.StartTask("Test Operation", func(ctx, task) error {
			// Task logging available in tests
			task.Infof("Running test operation")
			return performTestOperation()
		})

		result := task.WaitFor()
		assert.NoError(t, result.Error)
		assert.Equal(t, task.StatusSuccess, result.Status)
	}

# Advanced Features

# Task Dependencies

Tasks can depend on other tasks:

	prerequisite := task.StartTask("Load Configuration", loadConfig)

	mainTask := task.StartTaskWithOptions("Main Process", mainProcess,
		task.WithDependencies(prerequisite),
	)

	// mainTask won't start until prerequisite completes successfully

# Task Identity and Deduplication

Prevent duplicate tasks with identity tracking:

	task1 := task.StartTaskWithOptions("Download File", downloadFunc,
		task.WithIdentity("download-file-xyz"),
	)

	// This will return the existing task instead of creating a new one
	task2 := task.StartTaskWithOptions("Download File", downloadFunc,
		task.WithIdentity("download-file-xyz"),
	)

	// task1 and task2 reference the same underlying task

# Custom Styling and Themes

Tasks support custom visual themes:

	customTheme := api.Theme{
		Success: "text-blue-600",
		Error:   "text-purple-600",
		Warning: "text-orange-500",
	}

	manager := task.NewManager(task.WithTheme(customTheme))

# Graceful Shutdown

Handle interrupts gracefully:

	manager := task.NewManager(
		task.WithGracefulTimeout(30 * time.Second),
		task.WithInterruptHandler(func() {
			fmt.Println("Shutting down gracefully...")
		}),
	)

	// Manager will handle SIGINT/SIGTERM and allow running tasks to complete

The task package provides a complete solution for concurrent task execution with rich visual feedback,
comprehensive error handling, and flexible configuration options suitable for CLI tools, servers,
and batch processing applications.
*/
package task
