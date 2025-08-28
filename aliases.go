package clicky

import (
	"fmt"

	flanksourceContext "github.com/flanksource/commons/context"

	"github.com/flanksource/clicky/task"
)

type Context = flanksourceContext.Context

// Type aliases for backward compatibility
type (
	Task               = task.Task
	TaskManager        = task.Manager
	TaskGroup          = task.Group
	TaskStatus         = task.Status
	TaskOption         = task.Option
	TaskFunc           = task.TaskFunc[any]
	TaskResult         = task.TaskResult[any]
	TypedTask          = task.TypedTask[any]
	LogEntry           = task.LogEntry
	RetryConfig        = task.RetryConfig
	WaitResult         = task.WaitResult
	Waitable           = task.Waitable
	TaskManagerOptions = task.ManagerOptions
)

// Status constants
const (
	StatusPending   = task.StatusPending
	StatusRunning   = task.StatusRunning
	StatusSuccess   = task.StatusSuccess
	StatusFailed    = task.StatusFailed
	StatusWarning   = task.StatusWarning
	StatusCancelled = task.StatusCancelled
)

// Function aliases for backward compatibility
var (
	NewTaskManager                = task.NewManager
	NewTaskManagerWithConcurrency = task.NewManagerWithConcurrency
	NewTaskManagerWithOptions     = task.NewManagerWithOptions
	DefaultRetryConfig            = task.DefaultRetryConfig
	DefaultTaskManagerOptions     = task.DefaultManagerOptions
	WithTimeout                   = task.WithTimeout
	WithTaskTimeout               = task.WithTaskTimeout
	WithDependencies              = task.WithDependencies
	WithFunc                      = task.WithFunc
	WithModel                     = task.WithModel
	WithPrompt                    = task.WithPrompt
	WithRetryConfig               = task.WithRetryConfig
	WithPriority                  = task.WithPriority
	BindTaskManagerFlags          = task.BindManagerFlags
	BindTaskManagerPFlags         = task.BindManagerPFlags
)

// StartWithResultTyped creates and starts tracking a new task with generic typed result handling
func StartWithResultTyped[T any](tm *TaskManager, name string, taskFunc task.TaskFunc[T], opts ...TaskOption) *task.Task {
	// Wrap the typed function to work with the existing interface{} system
	wrappedFunc := func(ctx flanksourceContext.Context, task *task.Task) (interface{}, error) {
		result, err := taskFunc(ctx, task)
		return result, err
	}
	return tm.StartWithResult(name, wrappedFunc, opts...)
}

// GetResultTyped returns the stored result with type assertion
func GetResultTyped[T any](t *Task) (T, error) {
	result, err := t.GetResult()
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
