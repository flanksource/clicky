package task

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/text"
	"github.com/samber/lo"
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
	// StatusCancelled indicates the task was cancelled
	StatusCancelled Status = "cancelled"

	StatusPASS Status = "PASS"
	StatusFAIL Status = "FAIL"
	StatusERR  Status = "ERR"
	StatusSKIP Status = "SKIP"
)

func (s Status) String() string {
	return string(s)
}

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

func (s Status) Style() string {
	if s == StatusRunning {
		return "text-blue-500"
	}
	return s.Health().Style()
}

func (s Status) Apply(t api.Text) api.Text {
	t.Content = fmt.Sprintf("%s %s", s.Icon(), t.Content)
	t.Style = s.Style()
	return t
}

func (s Status) Pretty() api.Text {
	return api.Text{
		Content: s.Icon() + " " + s.String(),
		Style:   s.Style(),
	}
}

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
	Status       Status
	Duration     time.Duration
	Error        error
	TaskCount    int // Number of individual tasks (1 for Task, N for TaskGroup)
	SuccessCount int // Number of successful tasks
	FailureCount int // Number of failed tasks
	WarningCount int // Number of tasks with warnings
}

// LogEntry represents a log message from a task
type LogEntry struct {
	Level   logger.LogLevel
	Message string
	Time    time.Time
}

// RetryConfig holds configuration for task retry behavior
type RetryConfig struct {
	MaxRetries      int
	BaseDelay       time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	JitterFactor    float64
	RetryableErrors []string // Error message patterns that should trigger retries
}

// DefaultRetryConfig returns sensible default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      3,
		BaseDelay:       1 * time.Second,
		MaxDelay:        30 * time.Second,
		BackoffFactor:   2.0,
		JitterFactor:    0.1,
		RetryableErrors: []string{"timeout", "connection", "temporary", "rate limit", "429"},
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
	name           string
	modelName      string
	prompt         string
	status         Status
	dirty          atomic.Bool // Indicates if the task has been modified since last render
	progress       int
	maxValue       int
	startTime      time.Time
	endTime        time.Time
	manager        *Manager
	logs           []LogEntry
	cancel         context.CancelFunc
	ctx            flanksourceContext.Context
	flanksourceCtx flanksourceContext.Context
	timeout        time.Duration
	taskTimeout    time.Duration // Individual task timeout applied at execution time
	runFunc        func(flanksourceContext.Context, *Task) error
	err            error
	mu             sync.Mutex
	retryConfig    RetryConfig
	retryCount     int
	parent         *Group        // Reference to parent group (nil if ungrouped)
	doneChan       chan struct{} // Channel to signal task completion
	doneOnce       sync.Once     // Ensure done channel is closed only once
	dependencies   []*Task       // Tasks that must complete before this task can start
	completed      atomic.Bool   // Atomic flag for completion status
	priority       int           // Priority for queue ordering (lower = higher priority)
	enqueuedAt     time.Time     // Time when task was added to queue
	identity       string        // Unique identifier for task deduplication

	// Generic result storage
	result     interface{}
	resultType reflect.Type
}

// TypedTask provides typed access to task results
type TypedTask[T any] struct {
	*Task
}

type Taskable interface {
	GetTask() *Task
}

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
	}
	t.status = status
	t.dirty.Store(true) // Mark task as modified

}

// SetProgress updates the task's progress
func (t *Task) SetProgress(value, max int) {
	t.progress = value
	t.maxValue = max
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

func (t *Task) WaitTime() time.Duration {
	if t.endTime.IsZero() {
		return time.Since(t.startTime)
	}
	return t.endTime.Sub(t.startTime)
}

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
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for !t.completed.Load() {
		select {
		case <-t.ctx.Done():
			// Task was cancelled externally
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
				t.err = fmt.Errorf("task wait timeout after 30 seconds")
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
		displayName += fmt.Sprintf(" \"%s\"", truncatedPrompt)
	}

	text.Content = fmt.Sprintf("%50s %-10s", lo.Ellipsis(displayName, 50), duration)

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
			Content: fmt.Sprintf("\n\t  %s", log.Message),
			Style:   logStyle,
		})
	}

	return text
}
