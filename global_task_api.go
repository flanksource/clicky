package clicky

import (
	"os"
	"sync"
	"time"

	"github.com/flanksource/clicky/task"
)

var (

	// Phase tracking
	currentPhaseTask *Task
	phaseMutex       sync.Mutex
)

func StartTask[T any](name string, taskFunc task.TaskFunc[T], opts ...TaskOption) task.TypedTask[T] {
	return task.StartTask(name, taskFunc, opts...)
}

// WaitForGlobalCompletion waits for all global tasks to complete and returns exit code
func WaitForGlobalCompletion() int {
	return task.Wait()
}

// WaitForGlobalCompletionSilent waits for global tasks without displaying results
func WaitForGlobalCompletionSilent() int {
	return task.WaitSilent()
}

// SetGlobalInterruptHandler sets the interrupt handler for the global TaskManager
func SetGlobalInterruptHandler(fn func()) {
	task.SetInterruptHandler(fn)
}

// SetGlobalMaxConcurrency sets the maximum concurrency for the global TaskManager
func SetGlobalMaxConcurrency(max int) {
	task.SetMaxConcurrent(max)
}

// SetGlobalVerbose enables/disables verbose output for the global TaskManager
func SetGlobalVerbose(verbose bool) {
	task.SetVerbose(verbose)
}

// CancelAllGlobalTasks cancels all running global tasks
func CancelAllGlobalTasks() {

	task.CancelAll()
}

// ClearGlobalTasks removes completed tasks from the global TaskManager
func ClearGlobalTasks() {
	task.ClearTasks()
}

// GetGlobalTaskManagerStats returns stats about the global TaskManager
func GetGlobalTaskManagerStats() (total, running, completed, failed int) {

	// This would require adding a stats method to TaskManager
	// For now, return basic info
	return 0, 0, 0, 0 // TODO: Implement stats method
}

// RegisterGlobalExit ensures tasks are displayed when the program exits
func RegisterGlobalExit() {

	// This could be called at program startup to ensure tasks are shown on exit
	// Implementation would depend on how the program exit flow works
}

// SetGlobalSignalTimeout configures the graceful shutdown timeout
func SetGlobalSignalTimeout(timeout time.Duration) {
	task.SetGracefulTimeout(timeout)
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

	// Complete the previous phase if it exists
	if currentPhaseTask != nil {
		currentPhaseTask.Success()
		currentPhaseTask = nil
	}

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
