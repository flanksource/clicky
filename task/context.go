package task

import (
	"context"

	commonscontext "github.com/flanksource/commons/context"
)

// WithContext links task cancellation and deadlines to its owner before enqueue.
// Multiple owners (a run group and a request, for example) all remain effective.
func WithContext(parent context.Context) Option {
	if parent == nil {
		panic("task: parent context is required")
	}
	return func(t *Task) {
		previous, previousCancel := t.ctx, t.cancel
		ctx, cancel := context.WithCancelCause(parent)
		stop := context.AfterFunc(previous, func() { cancel(context.Cause(previous)) })
		if err := previous.Err(); err != nil {
			cancel(context.Cause(previous))
		}
		t.ctx = commonscontext.NewContext(ctx)
		t.flanksourceCtx = t.ctx
		t.cancel = func() {
			stop()
			cancel(context.Canceled)
			previousCancel()
		}
	}
}
