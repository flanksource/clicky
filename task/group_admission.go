package task

import (
	"context"
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
//
// A member that is not admitted is parked: it leaves the task queue and waits
// in the line until it is handed back: when a permit is released, the member
// ahead of it is admitted or retires, or it is cancelled. Re-enqueueing it on a delay
// instead made every admission wait out the delay, and the queue's head while
// delayed blocks every other task, so a group drained one member per delay
// whatever its concurrency.
type admissionLine struct {
	mu      sync.Mutex
	waiting []*Task
	// parked holds each parked member's stop for the watch that unparks it
	// when its context ends.
	parked map[*Task]func() bool
}

// enlist puts a task at the back of its group's line. Called once per task as
// it is enqueued, so line order is enqueue order.
func (l *admissionLine) enlist(t *Task) {
	l.mu.Lock()
	l.waiting = append(l.waiting, t)
	l.mu.Unlock()
}

// park takes t out of circulation until unparkLocked. A context that ends
// while it waits unparks it, so a worker retires it.
func (g *Group) parkLocked(t *Task) {
	l := &g.admission
	if l.parked == nil {
		l.parked = map[*Task]func() bool{}
	}
	l.parked[t] = context.AfterFunc(t.ctx, func() { g.unpark(t) })
}

// unparkLocked hands a parked t back to the task queue; one not parked is
// already queued or being dequeued.
func (g *Group) unparkLocked(t *Task) {
	l := &g.admission
	stop, ok := l.parked[t]
	if !ok {
		return
	}
	stop()
	delete(l.parked, t)
	t.manager.taskQueue.Enqueue(t)
}

// unpark hands a parked t back to the task queue: it was cancelled before it
// started, so a worker retires it.
func (g *Group) unpark(t *Task) {
	g.admission.mu.Lock()
	defer g.admission.mu.Unlock()
	g.unparkLocked(t)
}

// admit takes a group permit for t when no member ahead of it in the line could
// start, and reports whether it did. A task that is not admitted keeps its
// place and is parked, so it holds neither a worker slot nor a queue entry. An
// admitted one wakes the line, whose next member may take a permit still free.
func (g *Group) admit(t *Task) bool {
	g.admission.mu.Lock()
	defer g.admission.mu.Unlock()
	admitted := g.admitLocked(t)
	if admitted {
		g.wakeLocked()
	} else {
		g.parkLocked(t)
	}
	return admitted
}

func (g *Group) admitLocked(t *Task) bool {
	l := &g.admission
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
			// Dropped from the line, so nothing would wake it again.
			g.unparkLocked(member)
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

// release returns a member's permit and wakes the line.
func (g *Group) release() {
	g.admission.mu.Lock()
	defer g.admission.mu.Unlock()
	g.sem.Release(1)
	g.wakeLocked()
}

// wake re-checks the line after a member ahead in it retired.
func (g *Group) wake() {
	g.admission.mu.Lock()
	defer g.admission.mu.Unlock()
	g.wakeLocked()
}

// wakeLocked hands back to the queue every parked member that retired while
// it waited, so a worker does its retirement bookkeeping, and the first member
// that could start, which takes a free permit or parks again.
func (g *Group) wakeLocked() {
	for _, member := range g.admission.waiting {
		retired := retiredBeforeStart(member)
		if !retired && !dependenciesReady(member) {
			continue
		}
		g.unparkLocked(member)
		if !retired {
			return
		}
	}
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
