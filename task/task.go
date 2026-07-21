package task

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/text"
	"golang.org/x/sync/semaphore"

	"github.com/flanksource/clicky/api"
)

// Status represents the status of a task
type Status string

const (
	// StatusPending indicates the task is waiting to start
	StatusPending Status = "pending"
	// StatusRunning indicates the task is currently running
	StatusRunning Status = "running"
	// StatusSuccess indicates the task completed successfully
	StatusSuccess Status = "success"
	// StatusFailed indicates the task failed
	StatusFailed Status = "failed"
	// StatusWarning indicates the task completed with warnings
	StatusWarning Status = "warning"
	// StatusCancelled indicates the task was canceled
	StatusCancelled Status = "canceled"

	// StatusPASS indicates a test passed
	StatusPASS Status = "PASS"
	// StatusFAIL indicates a test failed
	StatusFAIL Status = "FAIL"
	// StatusERR indicates a test had an error
	StatusERR Status = "ERR"
	// StatusSKIP indicates a test was skipped
	StatusSKIP Status = "SKIP"
)

func (s Status) String() string {
	return string(s)
}

// Icon returns the emoji icon representation of the status
func (s Status) Icon() string {
	switch s {
	case StatusPending:
		return "⏳"
	case StatusRunning:
		return "⟳"
	case StatusSuccess, StatusPASS:
		return "✓"
	case StatusFailed, StatusFAIL:
		return "✗"
	case StatusWarning, StatusERR:
		return "⚠"
	case StatusCancelled, StatusSKIP:
		return "⊘"
	default:
		return ""
	}
}

// Style returns the CSS style class for the status
func (s Status) Style() string {
	if s == StatusRunning {
		return "text-blue-500"
	}
	return s.Health().Style()
}

// Apply applies the status icon and style to the given text, preserving any
// style classes (such as width/truncation directives) the caller has already
// set.
func (s Status) Apply(t api.Text) api.Text {
	t.Content = fmt.Sprintf("%s %s", s.Icon(), t.Content)
	return t.AppendStyle(s.Style())
}

// Pretty returns a pretty formatted text representation of the status
func (s Status) Pretty() api.Text {
	return api.Text{
		Content: s.Icon() + " " + s.String(),
		Style:   s.Style(),
	}
}

// Health converts the status to a health state
func (s Status) Health() Health {
	switch s {
	case StatusSuccess, StatusPASS:
		return HealthOK
	case StatusWarning, StatusSKIP, StatusCancelled:
		return HealthWarning
	case StatusFailed, StatusERR, StatusFAIL:
		return HealthError
	default:
		return HealthPending
	}
}

// Waitable represents something that can be waited on (Task or TaskGroup)
type Waitable interface {
	Name() string
	Status() Status
	WaitFor() *WaitResult
	Context() context.Context
	Cancel()
	Duration() time.Duration
	IsGroup() bool
}

// WaitResult contains unified result information
type WaitResult struct {
	Error        error
	Status       Status
	Duration     time.Duration
	TaskCount    int // Number of individual tasks (1 for Task, N for TaskGroup)
	SuccessCount int // Number of successful tasks
	FailureCount int // Number of failed tasks
	WarningCount int // Number of tasks with warnings
}

// RetryConfig holds configuration for task retry behavior
type RetryConfig struct {
	RetryableErrors []string // Error message patterns that should trigger retries
	BaseDelay       time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	JitterFactor    float64
	MaxRetries      int
}

// DefaultRetryConfig returns sensible default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		RetryableErrors: []string{"timeout", "connection", "temporary", "rate limit", "429"},
		BaseDelay:       1 * time.Second,
		MaxDelay:        30 * time.Second,
		BackoffFactor:   2.0,
		JitterFactor:    0.1,
		MaxRetries:      3,
	}
}

// TaskFunc is a generic task function that returns a typed result
type TaskFunc[T any] func(flanksourceContext.Context, *Task) (T, error)

// TaskResult holds a typed result and error
type TaskResult[T any] struct {
	Result T
	Error  error
}

// Task represents a single task being tracked by the TaskManager
type Task struct {
	// Pointers and interfaces (8 bytes each on 64-bit)
	manager         *Manager
	cancel          context.CancelFunc
	ctx             flanksourceContext.Context
	flanksourceCtx  flanksourceContext.Context
	runFunc         func(flanksourceContext.Context, *Task) error
	err             error
	parent          *Group        // Reference to parent group (nil if ungrouped)
	doneChan        chan struct{} // Channel to signal task completion
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
	status Status
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
	return t.ctx
}

// FlanksourceContext returns the task's flanksource context for logging
func (t *Task) FlanksourceContext() flanksourceContext.Context {
	return t.flanksourceCtx
}

// Cancel cancels the task
func (t *Task) Cancel() {
	t.mu.Lock()
	if t.status == StatusPending || t.status == StatusRunning {
		t.status = StatusCancelled
		t.endTime = time.Now()
		if t.cancel != nil {
			t.cancel()
		}
		t.dirty.Store(true)
		t.signalDone() // Signal task completion
		t.mu.Unlock()
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
}

// Errorf logs an error message
func (t *Task) Errorf(format string, args ...interface{}) {
	t.getBufferedLogger().Errorf(format, args...)
}

// Warnf logs a warning message
func (t *Task) Warnf(format string, args ...interface{}) {
	t.getBufferedLogger().Warnf(format, args...)
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
	return t.err
}

// IsOk returns true if the task completed successfully
func (t *Task) IsOk() bool {
	return t.err == nil && t.Status() == StatusSuccess
}

// Status returns the current task status
func (t *Task) Status() Status {
	t.mu.Lock()
	defer t.mu.Unlock()

	if health, ok := t.result.(HealthMixin); ok {
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

const (
	// waitForResultGrace bounds the wait for a task that has already reached a
	// terminal status but whose worker has not yet stored the result. That gap
	// is microseconds wide; the grace only exists so a wedged worker surfaces
	// as a returned zero value instead of an unbounded wait.
	waitForResultGrace = 5 * time.Second
	// waitForWarnAfter is how long WaitFor stays quiet before reporting that it
	// is still waiting. It doubles after each report, matching WaitForAllTasks.
	waitForWarnAfter = 30 * time.Second
)

// WaitFor waits for this specific task to complete and returns the result.
//
// The wait is bounded by the task's own deadline (WithTimeout / WithTaskTimeout)
// and by nothing else. WaitFor used to impose a hardcoded 300s deadline and,
// when it fired, rewrite the still-running task as Failed — which raced the
// task's real timeout (a 5m linter ties it exactly) and, worse, deadlocked:
// the error it built re-received from the already-drained one-shot timeout
// channel while holding t.mu, so the task never completed and its worker never
// got the lock back. A task that overruns is now reported, not rewritten.
func (t *Task) WaitFor() *WaitResult {
	// Poll for task completion using atomic flag
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	start := time.Now()
	warnAfter := waitForWarnAfter

	for !t.completed.Load() {
		t.mu.Lock()
		taskCtx := t.ctx
		t.mu.Unlock()
		select {
		case <-taskCtx.Done():
			// ctx.Done fires for two distinct reasons: a genuine external
			// cancellation while the task is still pending/running, OR the task
			// itself reaching a terminal status — SetStatus cancels t.ctx on
			// Success/Failed/Warning/Cancelled. In the latter case the result is
			// still being stored by runFunc (the task closure typically calls
			// t.Success() before `return result`), so bailing out here would
			// return the zero value before the result lands. Only treat ctx.Done
			// as a real cancellation when the status is still non-terminal.
			t.mu.Lock()
			terminal := t.status != StatusRunning && t.status != StatusPending
			if !terminal {
				t.status = StatusCancelled
				t.endTime = time.Now()
				t.completed.Store(true)
			}
			t.mu.Unlock()
			if !terminal {
				goto done
			}
			// Self-cancel during terminal SetStatus: ctx is now permanently
			// ready, so re-selecting on it would busy-spin. Wait on doneChan
			// (closed by the worker immediately after it stores the result and
			// flips completed) so we wake the instant the result lands, with a
			// fresh grace timer as a backstop.
			select {
			case <-t.doneChan:
			case <-time.After(waitForResultGrace):
			}
			goto done
		case <-ticker.C:
			if waited := time.Since(start); waited >= warnAfter {
				logger.Warnf("Still waiting for task %q (%s) after %s", t.Name(), t.Status(), waited.Round(time.Second))
				warnAfter *= 2
			}
		}
	}

done:

	t.mu.Lock()
	defer t.mu.Unlock()

	// Calculate duration without acquiring mutex (already held)
	var duration time.Duration
	if t.status != StatusPending && !t.enqueuedAt.IsZero() {
		endTime := t.endTime
		if t.status == StatusRunning {
			endTime = time.Now()
		}
		duration = endTime.Sub(t.startTime)
	}

	result := &WaitResult{
		Status:    t.status,
		Duration:  duration,
		Error:     t.err,
		TaskCount: 1, // Single task
	}

	// Count based on status
	switch t.status {
	case StatusSuccess:
		result.SuccessCount = 1
	case StatusFailed:
		result.FailureCount = 1
	case StatusWarning:
		result.WarningCount = 1
	case StatusPending, StatusRunning, StatusCancelled, StatusPASS, StatusFAIL, StatusERR, StatusSKIP:
		// These statuses don't contribute to specific counts
	}

	return result
}

// GetResult returns the stored result and error
func (t *Task) GetResult() (interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.result, t.err
}

// SetResult stores a result in the task
func (t *Task) SetResult(result interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.result = result
	if result != nil {
		t.resultType = reflect.TypeOf(result)
	}
}

// GetTypedResult retrieves the result with type assertion
func (t *Task) GetTypedResult(target interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.result == nil {
		return t.err
	}

	// Use reflection to set the target value
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	resultValue := reflect.ValueOf(t.result)
	targetElement := targetValue.Elem()

	if !resultValue.Type().AssignableTo(targetElement.Type()) {
		return fmt.Errorf("result type %T cannot be assigned to target type %T", t.result, target)
	}

	targetElement.Set(resultValue)
	return t.err
}

// Duration returns the task duration
func (t *Task) Duration() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status == StatusPending || t.enqueuedAt.IsZero() {
		return 0
	}

	endTime := t.endTime
	if t.status == StatusRunning {
		endTime = time.Now()
	}

	return endTime.Sub(t.startTime)
}

// EndTime returns when the task reached a terminal state, or the zero time if
// it is still pending/running.
func (t *Task) EndTime() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.endTime
}

// IsGroup returns false for Task
func (t *Task) IsGroup() bool {
	return false
}

// groupSem returns the concurrency semaphore of the task's parent group, or nil
// if the task is ungrouped or its group has no concurrency limit. Group.sem is
// assigned once in StartGroup before any task is added and never mutated, so no
// lock is needed.
func (t *Task) groupSem() *semaphore.Weighted {
	if t.parent == nil {
		return nil
	}
	return t.parent.sem
}

// getDuration returns formatted duration string
func (t *Task) getDuration() string {
	if t.status == StatusPending || t.startTime.IsZero() {
		return ""
	}
	// Note: This should be called with mutex already locked
	var end time.Time
	if t.endTime.IsZero() {
		end = time.Now()
	} else {
		end = t.endTime
	}

	return text.HumanizeDuration(end.Sub(t.startTime))
}

// Pretty returns a formatted text representation of the task with its full
// buffered log history.
func (t *Task) Pretty() api.Text {
	t.mu.Lock()
	defer t.mu.Unlock()
	text, _ := t.prettyWithLogOffset(0)
	return text
}

// prettyPlainDelta renders the task line plus only the log entries not yet
// emitted by a previous PlainRender tick, then advances the cursor. The buffer
// itself is preserved for Pretty(), snapshots, and the final tree.
func (t *Task) prettyPlainDelta() api.Text {
	t.mu.Lock()
	defer t.mu.Unlock()
	text, total := t.prettyWithLogOffset(t.plainLogsRendered)
	t.plainLogsRendered = total
	return text
}

// Logger interface implementation methods

// getBufferedLogger ensures the buffered logger is initialized
func (t *Task) getBufferedLogger() *logger.BufferedLogger {
	t.loggerOnce.Do(func() {
		t.bufferedLogger = logger.NewBufferedLogger(1000)
		if t.ctx.Logger != nil {
			t.bufferedLogger.SetLogLevel(t.ctx.Logger.GetLevel())
		}
	})
	return t.bufferedLogger
}

// Tracef logs a trace message (implements Logger interface)
func (t *Task) Tracef(format string, args ...interface{}) {
	t.getBufferedLogger().Tracef(format, args...)
}

// Fatalf logs a fatal message (implements Logger interface)
func (t *Task) Fatalf(format string, args ...interface{}) {
	t.getBufferedLogger().Fatalf(format, args...)
}

// WithValues returns a logger with additional key-value pairs (implements Logger interface)
func (t *Task) WithValues(keysAndValues ...interface{}) logger.Logger {
	return t.getBufferedLogger().WithValues(keysAndValues...)
}

// IsTraceEnabled checks if trace level is enabled (implements Logger interface)
func (t *Task) IsTraceEnabled() bool {
	return t.getBufferedLogger().IsTraceEnabled()
}

// IsDebugEnabled checks if debug level is enabled (implements Logger interface)
func (t *Task) IsDebugEnabled() bool {
	return t.getBufferedLogger().IsDebugEnabled()
}

// IsLevelEnabled checks if a specific level is enabled (implements Logger interface)
func (t *Task) IsLevelEnabled(level logger.LogLevel) bool {
	return t.getBufferedLogger().IsLevelEnabled(level)
}

// GetLevel returns the current log level (implements Logger interface)
func (t *Task) GetLevel() logger.LogLevel {
	return t.getBufferedLogger().GetLevel()
}

// ClearLogs clears all buffered logs for this task
func (t *Task) ClearLogs() {
	t.getBufferedLogger().ClearLogs()
}

// SetLogLevel sets the log level (implements Logger interface)
func (t *Task) SetLogLevel(level any) {
	t.getBufferedLogger().SetLogLevel(level)
}

// SetMinLogLevel sets the minimum log level (implements Logger interface)
func (t *Task) SetMinLogLevel(level any) {
	t.getBufferedLogger().SetMinLogLevel(level)
}

// V returns a verbose logger (implements Logger interface)
func (t *Task) V(level any) logger.Verbose {
	return t.getBufferedLogger().V(level)
}

// WithV returns a logger with verbosity level (implements Logger interface)
func (t *Task) WithV(level any) logger.Logger {
	return t.getBufferedLogger().WithV(level)
}

// Named returns a named logger (implements Logger interface - noop)
func (t *Task) Named(name string) logger.Logger {
	return t.getBufferedLogger().Named(name)
}

// WithoutName returns a logger without name (implements Logger interface - noop)
func (t *Task) WithoutName() logger.Logger {
	return t.getBufferedLogger().WithoutName()
}

// WithSkipReportLevel returns a logger with skip report level (implements Logger interface - noop)
func (t *Task) WithSkipReportLevel(i int) logger.Logger {
	return t.getBufferedLogger().WithSkipReportLevel(i)
}

// GetSlogLogger returns the slog logger (implements Logger interface - unsupported)
func (t *Task) GetSlogLogger() *slog.Logger {
	return t.getBufferedLogger().GetSlogLogger()
}
