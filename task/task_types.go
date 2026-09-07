package task

import (
	"context"
	"fmt"
	"github.com/flanksource/clicky/api"
	flanksourceContext "github.com/flanksource/commons/context"
	"time"
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
