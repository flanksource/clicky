package task

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/flanksource/commons/logger"
)

// Task represents a single task being tracked by the TaskManager
type Task struct {
	// Pointers and interfaces (8 bytes each on 64-bit)
	manager         *Manager
	cancel          context.CancelFunc
	ctx             flanksourceContext.Context
	flanksourceCtx  flanksourceContext.Context
	runFunc         func(flanksourceContext.Context, *Task) error
	err             error
	cancelErr       error
	parent          *Group        // Reference to parent group (nil if ungrouped)
	doneChan        chan struct{} // Channel to signal task completion
	drainedChan     chan struct{} // Channel closed after the callback has returned
	dependencies    []*Task       // Tasks that must complete before this task can start
	result          interface{}
	resultType      reflect.Type
	outputProvider  func() OutputSnapshot
	detailsProvider func() any
	controller      TaskController
	frozenOutput    *OutputSnapshot
	frozenDetails   any

	// Slices (24 bytes each on 64-bit)
	// logs removed - now stored only in bufferedLogger

	// Logger interface implementation
	bufferedLogger *logger.BufferedLogger

	// Structs
	mu          sync.Mutex
	doneOnce    sync.Once // Ensure done channel is closed only once
	drainedOnce sync.Once // Ensure drained channel is closed only once
	loggerOnce  sync.Once // Ensure bufferedLogger is initialized only once
	retryConfig RetryConfig

	// 8-byte aligned types
	startTime   time.Time
	endTime     time.Time
	timeout     time.Duration
	taskTimeout time.Duration // Individual task timeout applied at execution time
	enqueuedAt  time.Time     // Time when task was added to queue
	dirty       atomic.Bool   // Indicates if the task has been modified since last render
	completed   atomic.Bool   // Atomic flag for completion status
	background  atomic.Bool   // Excluded from drain waits; see SetBackground

	// Strings (16 bytes each on 64-bit)
	name        string
	description string
	modelName   string
	id          string
	prompt      string
	identity    string // Unique identifier for task deduplication

	// 4-byte types
	progress          int
	maxValue          int
	retryCount        int
	priority          int // Priority for queue ordering (lower = higher priority)
	plainLogsRendered int // buffered log entries already emitted by PlainRender; guarded by mu

	// Smaller types
	status            Status
	drainCancellation bool
}

// TypedTask provides typed access to task results
type TypedTask[T any] struct {
	*Task
}

// Taskable represents objects that can return a Task
type Taskable interface {
	GetTask() *Task
}

// GetTask returns the task itself
func (t *Task) GetTask() *Task {
	return t
}

// GetResult retrieves the typed result from a TypedTask
func (t TypedTask[T]) GetResult() (T, error) {
	// wait for task to complete
	wait := t.WaitFor()

	// get the result (if any)
	result, err := t.Task.GetResult()
	// Handle nil result explicitly
	if result != nil {
		typedResult, ok := result.(T)
		if !ok {
			return *new(T), fmt.Errorf("result type mismatch: expected %T, got %T", *new(T), result)
		}
		return typedResult, err
	}

	if wait != nil && wait.Error != nil {
		// wait error takes precedence over GetResult error
		return *new(T), wait.Error
	}
	return *new(T), err

}

// Identity returns the task's unique identifier for deduplication
func (t *Task) Identity() string {
	return t.identity
}

// ID returns the task's immutable UUID.
func (t *Task) ID() string {
	return t.id
}

// Context returns the task's context for cancellation
func (t *Task) Context() context.Context {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.ctx
}

// FlanksourceContext returns the task's flanksource context for logging
func (t *Task) FlanksourceContext() flanksourceContext.Context {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.flanksourceCtx
}

// Cancel cancels the task
func (t *Task) Cancel() {
	t.mu.Lock()
	if t.status == StatusPending || t.status == StatusRunning {
		// A task cancelled before it ever started will never run — the worker
		// skips it at dequeue. Record that here so a dependent dequeued in the
		// meantime sees a finished dependency rather than an unfinished
		// cancelled one, which checkDependencies would cancel along with it.
		neverStarted := t.status == StatusPending
		t.status = StatusCancelled
		t.endTime = time.Now()
		if t.cancel != nil {
			t.cancel()
		}
		t.cancelErr = context.Cause(t.ctx)
		if neverStarted {
			t.completed.Store(true)
		}
		t.dirty.Store(true)
		t.mu.Unlock()
		t.signalDone()
		if neverStarted {
			t.signalDrained()
		}
	} else {
		t.mu.Unlock()
	}
}

// signalDone safely closes the done channel exactly once
func (t *Task) signalDone() {
	t.doneOnce.Do(func() {
		close(t.doneChan)
	})
}

func (t *Task) signalDrained() {
	t.drainedOnce.Do(func() {
		close(t.drainedChan)
	})
}

func (t *Task) markCompleted() {
	t.completed.Store(true)
	t.signalDrained()
}

// Debugf logs a debug message (only shown in verbose mode)
func (t *Task) Debugf(format string, args ...interface{}) {
	t.getBufferedLogger().Debugf(format, args...)
}

// PopDirty checks and clears the dirty flag atomically
func (t *Task) PopDirty() bool {
	// Atomically check and reset dirty flag
	b := t.dirty.Load()
	t.dirty.Store(false)
	return b
}

// Infof logs an info message (only shown in verbose mode)
func (t *Task) Infof(format string, args ...interface{}) {
	t.getBufferedLogger().Infof(format, args...)
	t.markLogForStreaming()
}

// Errorf logs an error message
func (t *Task) Errorf(format string, args ...interface{}) {
	t.getBufferedLogger().Errorf(format, args...)
	t.markLogForStreaming()
}

// Warnf logs a warning message
func (t *Task) Warnf(format string, args ...interface{}) {
	t.getBufferedLogger().Warnf(format, args...)
	t.markLogForStreaming()
}

// markLogForStreaming flags the task dirty so the plain render loop emits a
// just-appended log line on its next tick instead of batching it until the
// next status transition. Called for Info and more-severe appends only —
// Debug/Trace lines stay batched.
func (t *Task) markLogForStreaming() {
	t.dirty.Store(true)
}

// SetName sets the task name
func (t *Task) SetName(name string) {
	t.mu.Lock()
	t.name = name
	t.mu.Unlock()
	t.dirty.Store(true) // Mark task as modified
}

// SetDescription sets the task description
func (t *Task) SetDescription(description string) {
	t.mu.Lock()
	t.description = description
	t.mu.Unlock()
	t.dirty.Store(true) // Mark task as modified
}

// Description returns the task description
func (t *Task) Description() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.description
}

// SetOutputProvider attaches live stdout/stderr to the task snapshot. Providers
// are evaluated outside the task lock and frozen when a managed task finishes.
func (t *Task) SetOutputProvider(provider func() OutputSnapshot) {
	t.mu.Lock()
	t.outputProvider = provider
	t.frozenOutput = nil
	t.mu.Unlock()
	t.dirty.Store(true)
}

// SetDetailsProvider attaches structured, JSON-serializable task details.
func (t *Task) SetDetailsProvider(provider func() any) {
	t.mu.Lock()
	t.detailsProvider = provider
	t.frozenDetails = nil
	t.mu.Unlock()
	t.dirty.Store(true)
}

// SetController replaces the task's live controller.
func (t *Task) SetController(controller TaskController) {
	t.mu.Lock()
	t.controller = controller
	t.mu.Unlock()
	t.dirty.Store(true)
}

func (t *Task) snapshotOutput() OutputSnapshot {
	t.mu.Lock()
	if t.frozenOutput != nil {
		output := *t.frozenOutput
		t.mu.Unlock()
		return output
	}
	provider := t.outputProvider
	t.mu.Unlock()
	if provider == nil {
		return OutputSnapshot{}
	}
	return provider()
}

func (t *Task) snapshotDetails() any {
	t.mu.Lock()
	if t.frozenDetails != nil {
		details := t.frozenDetails
		t.mu.Unlock()
		return details
	}
	provider := t.detailsProvider
	t.mu.Unlock()
	if provider == nil {
		return nil
	}
	return provider()
}

func (t *Task) freezeProviders() {
	output := t.snapshotOutput()
	details := t.snapshotDetails()
	t.mu.Lock()
	t.frozenOutput = &output
	t.frozenDetails = details
	t.outputProvider = nil
	t.detailsProvider = nil
	t.mu.Unlock()
}

// SetStatus updates the task's display name/status message
func (t *Task) SetStatus(status Status) {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch status {
	case StatusSuccess, StatusCancelled, StatusFailed, StatusWarning:
		t.endTime = time.Now()
		if t.cancel != nil {
			t.cancel()
			t.cancel = nil
		}
	case StatusPending, StatusRunning, StatusPASS, StatusFAIL, StatusERR, StatusSKIP:
		// These statuses don't require special cleanup
	}
	t.status = status
	t.dirty.Store(true) // Mark task as modified
}

// SetProgress updates the task's progress
func (t *Task) SetProgress(value, maximum int) {
	t.mu.Lock()
	t.progress = value
	t.maxValue = maximum
	t.mu.Unlock()
	t.dirty.Store(true) // Mark task as modified
}

// Progress returns the task's current progress value and maximum. A maximum of 0
// means the task has no bounded progress.
func (t *Task) Progress() (value, maximum int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.progress, t.maxValue
}

// Success marks the task as successfully completed
func (t *Task) Success() *Task {
	t.SetStatus(StatusSuccess)
	return t
}

// Failed marks the task as failed
func (t *Task) Failed() *Task {
	t.SetStatus(StatusFailed)
	return t
}

// FailedWithError marks the task as failed with an error
func (t *Task) FailedWithError(err error) (*Task, error) {
	// Log to bufferedLogger
	t.getBufferedLogger().Errorf("%s", err.Error())

	t.SetStatus(StatusFailed)
	return nil, nil
}

// Warning marks the task as completed with warnings
func (t *Task) Warning() *Task {
	t.SetStatus(StatusWarning)
	return t
}

// Fatal marks the task as failed and exits the program immediately
func (t *Task) Fatal(err error) {
	t.mu.Lock()
	t.status = StatusFailed
	t.err = err
	t.endTime = time.Now()
	if t.cancel != nil {
		t.cancel()
	}
	name := t.name
	t.mu.Unlock()

	if t.manager != nil {
		t.manager.stopRender()
	}

	logger.Fatalf("Fatal: %s: %v", name, err)
}

// Error returns the task's error if any
func (t *Task) Error() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.resultError()
}

// IsOk returns true if the task completed successfully
func (t *Task) IsOk() bool {
	return t.Error() == nil && t.Status() == StatusSuccess
}

// Status returns the current task status
func (t *Task) Status() Status {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.completed.Load() && t.status != StatusCancelled {
		health, ok := t.result.(HealthMixin)
		if !ok {
			return t.status
		}
		switch health.Health() {
		case HealthOK:
			t.status = StatusSuccess
		case HealthWarning:
			t.status = StatusWarning
		case HealthError:
			t.status = StatusFailed
		case HealthPending:
			t.status = StatusPending
		}
	}
	return t.status
}

// WaitTime returns how long the task waited before starting
func (t *Task) WaitTime() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.endTime.IsZero() {
		return time.Since(t.startTime)
	}
	return t.endTime.Sub(t.startTime)
}

// StartTime returns when the task started execution
func (t *Task) StartTime() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.startTime
}

// Name returns the task name
func (t *Task) Name() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.name
}

// SetBackground marks the task as long-lived: a server whose contract is to
// outlive any wait, rather than work a wait is entitled to drain. Waits skip
// such tasks entirely (see tasksDrained).
//
// Counting a long-lived server in a drain deadlocks every caller, because the
// two sides need opposite things: the wait blocks the work that would shut the
// server down, and the running server keeps the wait from returning. A
// supervised agent-provider process is the canonical case — it must stay alive
// across turns, so a commit hook that drains global tasks mid-run hangs forever.
//
// It is opt-in: a supervised process that IS the work (`gavel proc run`) must
// keep blocking the wait.
func (t *Task) SetBackground(background bool) {
	t.background.Store(background)
}

// IsBackground reports whether waits skip this task. See SetBackground.
func (t *Task) IsBackground() bool {
	return t.background.Load()
}
