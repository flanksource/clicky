package task

import (
	"context"
	"time"
)

type groupContextKey struct{}

// GroupFromContext returns the group that owns the task context, or nil.
func GroupFromContext(ctx context.Context) *Group {
	group, _ := ctx.Value(groupContextKey{}).(*Group)
	return group
}

// reopenAncestors clears terminal timestamps when descendant work is added,
// allowing persistence and retention to observe the next completion.
func (g *Group) reopenAncestors() {
	for current := g; current != nil; current = current.parent {
		current.mu.Lock()
		current.finishedAt = time.Time{}
		current.mu.Unlock()
	}
}

// WithGroupContext carries request values and cancellation into every member.
func WithGroupContext(ctx context.Context) TaskGroupOption {
	if ctx == nil {
		panic("task: group context is nil")
	}
	return func(g *Group) { g.ctx = ctx }
}

// WithParentGroup attaches a group to its owner's lifecycle and snapshot tree.
func WithParentGroup(parent *Group) TaskGroupOption {
	if parent == nil {
		panic("task: parent group is nil")
	}
	return func(g *Group) { g.parent = parent }
}

func (g *Group) childGroups() []*Group {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return append([]*Group(nil), g.children...)
}

func (g *Group) allTasks() []Taskable {
	items := g.GetTasks()
	for _, child := range g.childGroups() {
		items = append(items, child.allTasks()...)
	}
	return items
}

// memberStatuses aggregates direct tasks and child groups without closing
// pending children whose owners may still be scheduling their first task.
func (g *Group) memberStatuses() []Status {
	statuses := []Status{}
	for _, item := range g.GetTasks() {
		item.GetTask().cancelIfContextDone()
		statuses = append(statuses, item.GetTask().Status())
	}
	for _, child := range g.childGroups() {
		statuses = append(statuses, child.Status())
	}
	return statuses
}

// bindContext gives a group its own identity and cancellation scope while
// preserving request values and following its parent's cancellation.
func (g *Group) bindContext() {
	if g.parent != nil && g.ctx == nil {
		g.ctx = g.parent.Context()
	}
	if g.ctx == nil {
		g.ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(g.ctx)
	g.ctx, g.cancel = context.WithValue(ctx, groupContextKey{}, g), cancel
	if g.parent != nil {
		stop := context.AfterFunc(g.parent.Context(), cancel)
		g.cancel = func() { stop(); cancel() }
	}
}
