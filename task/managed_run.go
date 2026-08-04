package task

import (
	"fmt"
	"time"
)

// ManagedRun tracks externally-driven work without occupying a task worker.
// It is intended for supervisors and other long-lived lifecycle owners.
type ManagedRun struct {
	group *Group
	task  *Task
}

// StartManagedRun creates a one-task group already in the running state.
func StartManagedRun(name string, opts ...TaskGroupOption) *ManagedRun {
	group := StartGroup[any](name, opts...)
	t := global.newTask(name, WithGroup(group.Group))
	group.mu.RLock()
	controller := group.controller
	group.mu.RUnlock()
	if controller != nil {
		t.SetController(controller)
	}
	attachTaskableToGroup(t, t)
	now := time.Now()
	t.mu.Lock()
	t.status = StatusRunning
	t.startTime = now
	t.enqueuedAt = now
	t.mu.Unlock()
	global.mu.Lock()
	global.tasks = append(global.tasks, t)
	global.mu.Unlock()
	return &ManagedRun{group: group.Group, task: t}
}

func (r *ManagedRun) ID() string { return r.group.ID() }

func (r *ManagedRun) TaskID() string { return r.task.ID() }

func (r *ManagedRun) Task() *Task { return r.task }

// SetHref updates the route advertised by the run.
func (r *ManagedRun) SetHref(href string) {
	r.group.mu.Lock()
	r.group.metadata.Href = href
	r.group.mu.Unlock()
}

func (r *ManagedRun) SetOutputProvider(provider func() OutputSnapshot) {
	r.task.SetOutputProvider(provider)
}

func (r *ManagedRun) SetDetailsProvider(provider func() any) {
	r.group.mu.Lock()
	r.group.detailsProvider = provider
	r.group.frozenDetails = nil
	r.group.mu.Unlock()
	r.task.SetDetailsProvider(provider)
}

// Finish freezes live providers and transitions the run to a terminal status.
func (r *ManagedRun) Finish(status Status, err error) {
	switch status {
	case StatusSuccess, StatusFailed, StatusWarning, StatusCancelled:
	default:
		panic(fmt.Sprintf("managed run cannot finish with non-terminal status %q", status))
	}
	r.task.freezeProviders()
	r.group.freezeDetails()
	r.task.mu.Lock()
	if r.task.completed.Load() {
		r.task.mu.Unlock()
		return
	}
	r.task.err = err
	r.task.status = status
	r.task.endTime = time.Now()
	if r.task.cancel != nil {
		r.task.cancel()
		r.task.cancel = nil
	}
	r.task.completed.Store(true)
	r.task.dirty.Store(true)
	r.task.mu.Unlock()
	r.task.signalDone()
	r.group.observeTerminal(status, time.Now())
}
