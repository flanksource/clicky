package task

import "context"

type taskContextKey struct{}

// TaskFromContext returns the task associated with ctx, or nil outside a task.
// Callback contexts and Task.Context carry the owning task; a child task's
// context identifies the child without changing its parent's context.
func TaskFromContext(ctx context.Context) *Task {
	current, _ := ctx.Value(taskContextKey{}).(*Task)
	return current
}
