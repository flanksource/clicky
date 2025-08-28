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
	"github.com/flanksource/commons/text"
	"github.com/samber/lo"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
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

// Apply applies the status icon and style to the given text
func (s Status) Apply(t api.Text) api.Text {
	t.Content = fmt.Sprintf("%s %s", s.Icon(), t.Content)
	t.Style = s.Style()
	return t
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

// LogEntry represents a log message from a task
type LogEntry struct {
	Message string
	Time    time.Time
	Level   logger.LogLevel
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
	manager        *Manager
	cancel         context.CancelFunc
	ctx            flanksourceContext.Context
	flanksourceCtx flanksourceContext.Context
	runFunc        func(flanksourceContext.Context, *Task) error
	err            error
	parent         *Group        // Reference to parent group (nil if ungrouped)
	doneChan       chan struct{} // Channel to signal task completion
	dependencies   []*Task       // Tasks that must complete before this task can start
	result         interface{}
	resultType     reflect.Type

	// Slices (24 bytes each on 64-bit)
	logs []LogEntry

	// Structs
	mu          sync.Mutex
	doneOnce    sync.Once // Ensure done channel is closed only once
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
	name      string
	modelName string
	prompt    string
	identity  string // Unique identifier for task deduplication

	// 4-byte types
	progress   int
	maxValue   int
	retryCount int
	priority   int // Priority for queue ordering (lower = higher priority)

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
	wait := t.Task.WaitFor()
	if wait != nil && wait.Error != nil {
		return *new(T), wait.Error
	}

	result, err := t.Task.GetResult()
	if err != nil {
		return *new(T), err
	}

	// Handle nil result explicitly
	if result == nil {
		return *new(T), nil
	}

	typedResult, ok := result.(T)
	if !ok {
		return *new(T), fmt.Errorf("result type mismatch: expected %T, got %T", *new(T), result)
	}
	return typedResult, nil
}

// Identity returns the task's unique identifier for deduplication
func (t *Task) Identity() string {
	return t.identity
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
	message := fmt.Sprintf(format, args...)
	t.logs = append(t.logs, LogEntry{
		Level:   logger.Debug,
		Message: message,
		Time:    time.Now(),
	})
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
	message := fmt.Sprintf(format, args...)
	t.logs = append(t.logs, LogEntry{
		Level:   logger.Info,
		Message: message,
		Time:    time.Now(),
	})
}

// Errorf logs an error message
func (t *Task) Errorf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	t.logs = append(t.logs, LogEntry{
		Level:   logger.Error,
		Message: message,
		Time:    time.Now(),
	})
}

// Warnf logs a warning message
func (t *Task) Warnf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	t.logs = append(t.logs, LogEntry{
		Level:   logger.Warn,
		Message: message,
		Time:    time.Now(),
	})
}

// SetName sets the task name
func (t *Task) SetName(name string) {
	t.name = name
	t.dirty.Store(true) // Mark task as modified
}

// SetStatus updates the task's display name/status message
func (t *Task) SetStatus(status Status) {
	switch status {
	case StatusSuccess, StatusCancelled, StatusFailed:
		t.endTime = time.Now()
		if t.cancel != nil {
			t.cancel()
			t.cancel = nil
		}
	case StatusPending, StatusRunning, StatusWarning, StatusPASS, StatusFAIL, StatusERR, StatusSKIP:
		// These statuses don't require special cleanup
	}
	t.status = status
	t.dirty.Store(true) // Mark task as modified
}

// SetProgress updates the task's progress
func (t *Task) SetProgress(value, maximum int) {
	t.progress = value
	t.maxValue = maximum
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
	t.logs = append(t.logs, LogEntry{
		Level:   logger.Error,
		Message: err.Error(),
		Time:    time.Now(),
	})

	t.SetStatus(StatusFailed)
	return t, err
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
		t.manager.mu.Lock()
		t.manager.stopRender <- true
		t.manager.mu.Unlock()
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
	if t.endTime.IsZero() {
		return time.Since(t.startTime)
	}
	return t.endTime.Sub(t.startTime)
}

// StartTime returns when the task started execution
func (t *Task) StartTime() time.Time {
	return t.startTime
}

// Name returns the task name
func (t *Task) Name() string {
	return t.name
}

// WaitFor waits for this specific task to complete and returns the result
func (t *Task) WaitFor() *WaitResult {
	// Poll for task completion using atomic flag
	timeout := time.After(300 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for !t.completed.Load() {
		select {
		case <-t.ctx.Done():
			// Task was canceled externally
			t.mu.Lock()
			if t.status == StatusRunning || t.status == StatusPending {
				t.status = StatusCancelled
				t.endTime = time.Now()
				t.completed.Store(true)
			}
			t.mu.Unlock()
			goto done
		case <-timeout:
			// Timeout fallback to prevent infinite waiting
			t.mu.Lock()
			if t.status == StatusRunning || t.status == StatusPending {
				t.status = StatusFailed
				t.err = fmt.Errorf("task wait timeout after %s", <-timeout)
				t.endTime = time.Now()
				t.completed.Store(true)
			}
			t.mu.Unlock()
			goto done
		case <-ticker.C:
			// Continue polling
		}
	}

done:

	t.mu.Lock()
	defer t.mu.Unlock()

	result := &WaitResult{
		Status:    t.status,
		Duration:  t.Duration(),
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

	if t.err != nil {
		return t.err
	}

	if t.result == nil {
		return nil
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
	return nil
}

// Duration returns the task duration
func (t *Task) Duration() time.Duration {
	if t.status == StatusPending || t.enqueuedAt.IsZero() {
		return 0
	}

	endTime := t.endTime
	if t.status == StatusRunning {
		endTime = time.Now()
	}

	return endTime.Sub(t.startTime)
}

// IsGroup returns false for Task
func (t *Task) IsGroup() bool {
	return false
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

// Pretty returns a formatted text representation of the task
func (t *Task) Pretty() api.Text {
	if pretty, ok := t.result.(formatters.PrettyMixin); ok {
		return pretty.Pretty()
	}

	var text api.Text

	duration := t.getDuration()
	displayName := t.name
	if t.modelName != "" {
		displayName = t.modelName + " " + displayName
	}
	if t.prompt != "" {
		truncatedPrompt := t.prompt
		displayName += fmt.Sprintf(" %q", truncatedPrompt)
	}

	text.Content = fmt.Sprintf("%s %-10s", lo.Ellipsis(displayName, api.GetTerminalWidth()-10), duration)

	text = t.Status().Apply(text)

	level := t.ctx.Logger.GetLevel()
	// Add logs as children if present
	logs := t.logs
	if len(logs) > 5 {
		logs = logs[len(logs)-5:]
	}
	for _, log := range logs {
		if level <= log.Level {
			continue
		}
		var logStyle string

		switch log.Level {
		case logger.Error:
			logStyle = "text-red-600"
		case logger.Warn:
			logStyle = "text-yellow-600"
		default:
			logStyle = "text-gray-600"
		}

		text.Children = append(text.Children, api.Text{
			Content: fmt.Sprintf("\n\t%s", lo.Ellipsis(log.Message, 500)),
			Style:   logStyle,
		})
	}

	return text
}
