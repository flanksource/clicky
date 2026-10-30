package task

import (
	"maps"

	"github.com/flanksource/clicky/prompt"
	flanksourceContext "github.com/flanksource/commons/context"
)

// promptScoped labels prompts raised under ctx with this task's id, keeping the
// caller's Owner/Kind/Labels, so the task's snapshot can list them. A nested
// task overwrites the label: a prompt belongs to the innermost running task.
func (t *Task) promptScoped(ctx flanksourceContext.Context) flanksourceContext.Context {
	scope := prompt.ScopeFrom(ctx)
	labels := maps.Clone(scope.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	labels[prompt.TaskLabel] = t.ID()
	scope.Labels = labels
	ctx.Context = prompt.WithScope(ctx.Context, scope)
	return ctx
}

// pendingPrompts lists the prompts waiting on this task. Only a running task can
// be blocked on one, so other states skip the store scan.
func pendingPrompts(t *Task) []prompt.PromptSnapshot {
	manager := prompt.GlobalManager()
	if manager == nil || t.Status() != StatusRunning {
		return nil
	}
	return manager.List(prompt.Filter{
		State:  string(prompt.StatePending),
		Labels: map[string]string{prompt.TaskLabel: t.ID()},
	})
}
