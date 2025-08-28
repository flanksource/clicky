package clicky

import (
	"os"
	"sync"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"

	"github.com/flanksource/clicky/task"
)

var (
	globalTaskManager *TaskManager
	taskManagerOnce   sync.Once
	initialized       = false

	// Phase tracking
	currentPhaseTask *Task
	phaseMutex       sync.Mutex
)

// initGlobalTaskManager initializes the global TaskManager with default settings
func initGlobalTaskManager() {
	taskManagerOnce.Do(func() {
		globalTaskManager = NewTaskManagerWithConcurrency(10)
		// Signal handling is registered automatically in NewManagerWithConcurrency
		initialized = true
	})
}

// UseGlobalTaskManager configures the global task manager with options
func UseGlobalTaskManager(opts task.ManagerOptions) {
	if !initialized {
		initGlobalTaskManager()
	}
	task.Global = globalTaskManager
	globalTaskManager.SetMaxConcurrent(opts.MaxConcurrent)
	globalTaskManager.SetGracefulTimeout(opts.GracefulTimeout)
	globalTaskManager.SetNoProgress(opts.NoProgress)
	globalTaskManager.SetNoColor(opts.NoColor)
	globalTaskManager.SetRetryConfig(RetryConfig{
		MaxRetries:      opts.MaxRetries,
		BaseDelay:       opts.RetryDelay,
		BackoffFactor:   2.0,
		JitterFactor:    0.1,
		RetryableErrors: []string{"timeout", "connection", "temporary", "rate limit", "429"},
		MaxDelay:        opts.RetryDelay * 3,
	})
}

func StartTask[T any](name string, taskFunc task.TaskFunc[T], opts ...TaskOption) task.TypedTask[T] {
	return task.StartTask(name, taskFunc, opts...)
}

// StartGlobalTask starts a new task using the global TaskManager
// Deprecated: Use StartTask instead.
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
