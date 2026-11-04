package task

import "context"

type groupContextKey struct{}

func GroupFromContext(ctx context.Context) *Group {
	group, _ := ctx.Value(groupContextKey{}).(*Group)
	return group
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
