package task

import (
	"context"
	"errors"
	"time"
)

// Step is work a running task does on its own goroutine, shown as a task of
// its own in the group: a phase with its own status and failure that never
// occupies a worker or a group concurrency permit, so it cannot wait behind the
// task that runs it.
type Step struct {
	task *Task
}

// StartStep adds a running step to the group. The caller runs the step's work
// itself and must Finish the step.
func (g *Group) StartStep(name string) *Step {
	t := g.manager.newTask(name, WithGroup(g))
	attachTaskableToGroup(t, t)
	now := time.Now()
	t.mu.Lock()
	t.status = StatusRunning
	t.startTime = now
	t.enqueuedAt = now
	t.mu.Unlock()
	t.dirty.Store(true)
	g.manager.mu.Lock()
	g.manager.tasks = append(g.manager.tasks, t)
	g.manager.mu.Unlock()
	return &Step{task: t}
}

// Task returns the step's task, for logging and progress.
func (s *Step) Task() *Task { return s.task }

// Finish records the step's outcome: success for a nil error, cancelled for a
// cancelled or expired context, failed otherwise. A step its group cancelled
// stays cancelled, and only the first Finish counts.
func (s *Step) Finish(err error) {
	t := s.task
	t.mu.Lock()
	if t.completed.Load() {
		t.mu.Unlock()
		return
	}
	switch {
	case t.status == StatusCancelled:
	case err == nil:
		t.status = StatusSuccess
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		t.status = StatusCancelled
	default:
		t.status = StatusFailed
	}
	t.err = err
	t.endTime = time.Now()
	if t.cancel != nil {
		t.cancel()
	}
	t.completed.Store(true)
	t.dirty.Store(true)
	t.mu.Unlock()
	t.signalDone()
	t.signalDrained()
	observeGroupTerminal(t.parent)
}
