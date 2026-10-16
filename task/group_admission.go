package task

import (
	"fmt"
	"slices"
	"sync"
)

// admissionLine is the order in which a concurrency-limited group's members
// take its permits: the order they were enqueued. Without it the permit goes to
// whichever member a free worker happens to dequeue next, so a group of
// concurrency 1 runs its members in an arbitrary order.
//
// A member holds its place only while it could start. One still waiting on a
// dependency, or whose dependency failed, is passed over: holding the line for
// it would stall the group behind work outside it, and deadlock it outright
// when the dependency is a later member of the same group. The line therefore
// orders the group's dependency-ready members. Members that were cancelled or
// already retired are dropped as the line is walked.
type admissionLine struct {
	mu      sync.Mutex
	waiting []*Task
}

// enlist puts a task at the back of its group's line. Called once per task as
// it is enqueued, so line order is enqueue order.
func (l *admissionLine) enlist(t *Task) {
	l.mu.Lock()
	l.waiting = append(l.waiting, t)
	l.mu.Unlock()
}

// admit takes a group permit for t when no member ahead of it in the line could
// start, and reports whether it did. A task that is not admitted keeps its
// place; the worker re-enqueues it rather than holding a worker slot.
func (g *Group) admit(t *Task) bool {
	l := &g.admission
	l.mu.Lock()
	defer l.mu.Unlock()
	kept := l.waiting[:0]
	defer func() {
		clear(l.waiting[len(kept):])
		l.waiting = kept
	}()
	for i, member := range l.waiting {
		if member == t {
			admitted := g.sem.TryAcquire(1)
			if admitted {
				kept = append(kept, l.waiting[i+1:]...)
			} else {
				kept = append(kept, l.waiting[i:]...)
			}
			return admitted
		}
		if retiredBeforeStart(member) {
			continue
		}
		kept = append(kept, member)
		if dependenciesReady(member) {
			kept = append(kept, l.waiting[i+1:]...)
			return false
		}
	}
	panic(fmt.Sprintf("task %q dequeued for group %q but missing from its admission line", t.name, g.name))
}

// retiredBeforeStart reports whether a waiting member will never take a permit
// because it was cancelled or already finished while queued.
func retiredBeforeStart(t *Task) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.completed.Load() || t.status == StatusCancelled || t.ctx.Err() != nil
}

// dependenciesReady reports whether every dependency of t has finished without
// failing — the condition under which checkDependencies lets t start. It has no
// side effects: a failed dependency cancels t only when t itself is dequeued.
func dependenciesReady(t *Task) bool {
	return !slices.ContainsFunc(t.dependencies, func(dep *Task) bool {
		if dep == nil {
			return false
		}
		dep.mu.Lock()
		failed := dep.status == StatusFailed
		dep.mu.Unlock()
		return failed || !dep.completed.Load()
	})
}
