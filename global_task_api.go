package clicky

import (
	"fmt"
	"os"
	"sync"
	"time"
	
	flanksourceContext "github.com/flanksource/commons/context"
)

var (
	globalTaskManager *TaskManager
	taskManagerOnce   sync.Once
	initialized       = false
	
	// Phase tracking
	currentPhaseTask  *Task
	phaseMutex        sync.Mutex
)

// initGlobalTaskManager initializes the global TaskManager with default settings
func initGlobalTaskManager() {
	taskManagerOnce.Do(func() {
		globalTaskManager = NewTaskManagerWithConcurrency(10)
		// Register signal handling for graceful shutdown
		globalTaskManager.registerSignalHandling()
		initialized = true
	})
}

// StartGlobalTask starts a new task using the global TaskManager
func StartGlobalTask(name string, opts ...TaskOption) *Task {
	initGlobalTaskManager()
	return globalTaskManager.Start(name, opts...)
}

// WaitForGlobalCompletion waits for all global tasks to complete and returns exit code
func WaitForGlobalCompletion() int {
	if !initialized {
		return 0 // No tasks were started
	}
	return globalTaskManager.Wait()
}

// WaitForGlobalCompletionSilent waits for global tasks without displaying results
func WaitForGlobalCompletionSilent() int {
	if !initialized {
		return 0 // No tasks were started
	}
	return globalTaskManager.WaitSilent()
}

// SetGlobalInterruptHandler sets the interrupt handler for the global TaskManager
func SetGlobalInterruptHandler(fn func()) {
	initGlobalTaskManager()
	globalTaskManager.SetInterruptHandler(fn)
}

// SetGlobalMaxConcurrency sets the maximum concurrency for the global TaskManager
func SetGlobalMaxConcurrency(max int) {
	initGlobalTaskManager()
	globalTaskManager.SetMaxConcurrent(max)
}

// SetGlobalVerbose enables/disables verbose output for the global TaskManager
func SetGlobalVerbose(verbose bool) {
	initGlobalTaskManager()
	globalTaskManager.SetVerbose(verbose)
}

// CancelAllGlobalTasks cancels all running global tasks
func CancelAllGlobalTasks() {
	if !initialized {
		return
	}
	globalTaskManager.CancelAll()
}

// ClearGlobalTasks removes completed tasks from the global TaskManager
func ClearGlobalTasks() {
	if !initialized {
		return
	}
	globalTaskManager.ClearTasks()
}

// GetGlobalTaskManagerStats returns stats about the global TaskManager
func GetGlobalTaskManagerStats() (total, running, completed, failed int) {
	if !initialized {
		return 0, 0, 0, 0
	}
	// This would require adding a stats method to TaskManager
	// For now, return basic info
	return 0, 0, 0, 0 // TODO: Implement stats method
}

// RegisterGlobalExit ensures tasks are displayed when the program exits
func RegisterGlobalExit() {
	if !initialized {
		return
	}
	// This could be called at program startup to ensure tasks are shown on exit
	// Implementation would depend on how the program exit flow works
}

// IsGlobalTaskManagerInitialized returns whether the global TaskManager is initialized
func IsGlobalTaskManagerInitialized() bool {
	return initialized
}

// GetGlobalTaskManager returns the global TaskManager instance (for backward compatibility)
// This function should be avoided in new code - use the API functions instead
func GetGlobalTaskManager() *TaskManager {
	initGlobalTaskManager()
	return globalTaskManager
}

// SetGlobalSignalTimeout configures the graceful shutdown timeout
func SetGlobalSignalTimeout(timeout time.Duration) {
	initGlobalTaskManager()
	globalTaskManager.SetGracefulTimeout(timeout)
}

// ExitWithGlobalTaskSummary displays task summary and exits with appropriate code
func ExitWithGlobalTaskSummary() {
	exitCode := WaitForGlobalCompletion()
	os.Exit(exitCode)
}

// StartGlobalPhase starts a new phase tracking task
func StartGlobalPhase(phaseName string) *Task {
	phaseMutex.Lock()
	defer phaseMutex.Unlock()
	
	initGlobalTaskManager()
	
	// Complete the previous phase if it exists
	if currentPhaseTask != nil {
		currentPhaseTask.Success()
		currentPhaseTask = nil
	}
	
	// Start new phase task
	currentPhaseTask = globalTaskManager.Start(
		phaseName,
		WithFunc(func(ctx flanksourceContext.Context, t *Task) error {
			t.Infof("Starting %s phase", phaseName)
			// Set progress as indeterminate (spinner)
			t.SetProgress(0, 0)
			
			// This task will run until CompleteGlobalPhase is called
			select {
			case <-t.Context().Done():
				return t.Context().Err()
			}
		}),
	)
	
	return currentPhaseTask
}

// UpdateGlobalPhaseProgress updates the progress of the current phase
func UpdateGlobalPhaseProgress(message string) {
	phaseMutex.Lock()
	defer phaseMutex.Unlock()
	
	if currentPhaseTask != nil {
		currentPhaseTask.Infof("%s", message)
	}
}

// CompleteGlobalPhase marks the current phase as completed
func CompleteGlobalPhase() {
	phaseMutex.Lock()
	defer phaseMutex.Unlock()
	
	if currentPhaseTask != nil {
		currentPhaseTask.Success()
		currentPhaseTask = nil
	}
}

// StartGlobalGroup creates a new task group using the global TaskManager
func StartGlobalGroup(name string) *TaskGroup {
	initGlobalTaskManager()
	return globalTaskManager.StartGroup(name)
}

// StartGlobalTaskInGroup creates a task within an existing group using the global TaskManager
func StartGlobalTaskInGroup(group *TaskGroup, name string, opts ...TaskOption) *Task {
	initGlobalTaskManager()
	return globalTaskManager.StartTaskInGroup(group, name, opts...)
}

// StartGlobalTaskWithResult creates a task with result handling using the global TaskManager
func StartGlobalTaskWithResult(name string, taskFunc func(flanksourceContext.Context, *Task) (interface{}, error), opts ...TaskOption) *Task {
	initGlobalTaskManager()
	return globalTaskManager.StartWithResult(name, taskFunc, opts...)
}

// Helper functions for common result patterns

// StartGlobalTaskReturning is a helper that creates a typed wrapper for cleaner syntax
func StartGlobalTaskReturning[T any](name string, taskFunc func(flanksourceContext.Context, *Task) (T, error), opts ...TaskOption) *Task {
	return StartGlobalTaskWithResult(name, func(ctx flanksourceContext.Context, t *Task) (interface{}, error) {
		result, err := taskFunc(ctx, t)
		return result, err
	}, opts...)
}

// GetTaskResult is a helper to get typed results from a task
func GetTaskResult[T any](task *Task) (T, error) {
	result, err := task.GetResult()
	var zero T
	if err != nil {
		return zero, err
	}
	if result == nil {
		return zero, nil
	}
	if typedResult, ok := result.(T); ok {
		return typedResult, nil
	}
	return zero, fmt.Errorf("result type mismatch: expected %T, got %T", zero, result)
}