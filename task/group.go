package task

import (
	"context"
	"sync"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
	"golang.org/x/sync/semaphore"
)

// GroupMetadata is the queryable metadata attached to a run (task group). Kind
// classifies the run (e.g. "sql-fix", "test-run"), Labels carry arbitrary
// key/value facets for filtering, and Owner identifies who started it. All
// fields are optional.
type GroupMetadata struct {
	Kind   string            `json:"kind,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
	Owner  string            `json:"owner,omitempty"`
}

// Group represents a group of tasks that can be managed collectively
type Group struct {
	name        string
	id          string     // stable unique id; distinct from name for registry drill-down
	Items       []Taskable // Can contain Tasks or nested Groups
	startTime   time.Time
	finishedAt  time.Time // set lazily the first time the group is observed terminal
	metadata    GroupMetadata
	manager     *Manager
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
	concurrency int
	sem         *semaphore.Weighted // Semaphore for concurrency control
}

type TaskGroupOption func(group *Group)

func WithConcurrency(concurrency int) TaskGroupOption {
	return func(group *Group) {
		group.concurrency = concurrency
	}
}

// WithGroupID sets a caller-supplied stable id for the group. When unset,
// StartGroup assigns a uuid.
func WithGroupID(id string) TaskGroupOption {
	return func(group *Group) {
		group.id = id
	}
}

// WithKind classifies the run for the task-manager listing/filtering.
func WithKind(kind string) TaskGroupOption {
	return func(group *Group) {
		group.metadata.Kind = kind
	}
}

// WithLabels attaches filterable key/value facets to the run.
func WithLabels(labels map[string]string) TaskGroupOption {
	return func(group *Group) {
		group.metadata.Labels = labels
	}
}

// WithOwner records who started the run.
func WithOwner(owner string) TaskGroupOption {
	return func(group *Group) {
		group.metadata.Owner = owner
	}
}

// ID returns the group's stable unique id.
func (g *Group) ID() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.id
}

// Metadata returns a copy of the group's metadata.
func (g *Group) Metadata() GroupMetadata {
	g.mu.RLock()
	defer g.mu.RUnlock()
	md := GroupMetadata{Kind: g.metadata.Kind, Owner: g.metadata.Owner}
	if g.metadata.Labels != nil {
		md.Labels = make(map[string]string, len(g.metadata.Labels))
		for k, v := range g.metadata.Labels {
			md.Labels[k] = v
		}
	}
	return md
}

// StartedAt returns the group's start time.
func (g *Group) StartedAt() time.Time {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.startTime
}

// FinishedAt returns the time the group first became terminal, or the zero
// time if it is still running/pending. It is recorded lazily by observeTerminal
// (called from snapshotting) so it does not depend on a WaitFor caller.
func (g *Group) FinishedAt() time.Time {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.finishedAt
}

// observeTerminal records finishedAt the first time the group is seen in a
// terminal status. Idempotent; safe to call on every snapshot.
func (g *Group) observeTerminal(status Status, now time.Time) {
	switch status {
	case StatusRunning, StatusPending:
		return
	}
	g.mu.Lock()
	if g.finishedAt.IsZero() {
		g.finishedAt = now
	}
	g.mu.Unlock()
}

func (g *Group) GetTasks() []Taskable {
	return g.Items
}

type TypedGroup[T any] struct {
	*Group
}

// Add adds a Waitable item (Task or Group) to this group
func (g TypedGroup[T]) Add(name string, taskFunc func(flanksourceContext.Context, *Task) (T, error), opts ...Option) TypedTask[T] {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Concurrency is enforced by the worker at dequeue time via the group's
	// semaphore (see worker.run / Task.groupSem); wrapping the task function to
	// acquire here would double-acquire the same permit (deadlock at N=1).
	//
	// The parent must be set BEFORE StartTask enqueues the task, otherwise a
	// worker could dequeue and run it ungated while parent is still nil. The
	// withParent option is applied inside newTask, before enqueue; it is
	// prepended so a caller-supplied option cannot override it.
	task := StartTask(name, taskFunc, append([]Option{withParent(g.Group)}, opts...)...)

	// Add to the group's items
	g.Items = append(g.Items, task)

	// Update start time if this is the first item or it started earlier
	task.mu.Lock()
	taskStartTime := task.startTime
	task.mu.Unlock()

	if g.startTime.IsZero() || taskStartTime.Before(g.startTime) {
		g.startTime = taskStartTime
	}
	return task
}

// GetResults waits for all tasks in the group and returns typed results
func (g TypedGroup[T]) GetResults() (map[TypedTask[T]]T, error) {
	results := make(map[TypedTask[T]]T)
	for _, item := range g.Items {
		switch v := item.(type) {
		case TypedTask[T]:
			v.WaitFor()
			r, err := v.GetResult()
			if err != nil {
				return nil, err
			}
			results[v] = r
		}
	}

	return results, nil
}

// Name returns the group name
func (g *Group) Name() string {
	return g.name
}

func (g *Group) Status() Status {
	if len(g.Items) == 0 {
		return StatusPending
	}

	hasRunning := false
	hasWarning := false
	hasFailed := false
	allCompleted := true

	for _, item := range g.Items {
		status := item.GetTask().Status()
		switch status {
		case StatusRunning:
			hasRunning = true
			allCompleted = false
		case StatusPending:
			allCompleted = false
		case StatusFailed:
			hasFailed = true
		case StatusWarning:
			hasWarning = true
		case StatusCancelled:
			hasFailed = true
		}
	}

	if hasRunning {
		return StatusRunning
	}
	if !allCompleted {
		return StatusPending
	}
	if hasFailed {
		return StatusFailed
	}
	if hasWarning {
		return StatusWarning
	}
	return StatusSuccess
}

// WaitFor waits for all child items to complete and returns aggregate results
// This version handles dynamically added tasks by continuously checking for new tasks
func (g *TypedGroup[T]) WaitFor() *WaitResult {
	result := &WaitResult{}

	// Keep track of the last known task count
	lastCount := -1
	stableIterations := 0
	const requiredStableIterations = 3 // Number of iterations with no new tasks before considering complete

	for {
		// Get current count of tasks
		g.mu.RLock()
		currentCount := len(g.Items)
		g.mu.RUnlock()

		// Check if we have new tasks
		if currentCount != lastCount {
			lastCount = currentCount
			stableIterations = 0
			// Small delay to allow more tasks to be queued
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// Check if all current tasks are complete
		allComplete := true
		hasRunning := false

		g.mu.RLock()
		for _, item := range g.Items {
			status := item.GetTask().Status()
			if status == StatusPending || status == StatusRunning {
				allComplete = false
				if status == StatusRunning {
					hasRunning = true
				}
				break
			}
		}
		g.mu.RUnlock()

		if allComplete {
			stableIterations++
			if stableIterations >= requiredStableIterations {
				// All tasks are complete and no new tasks have been added
				break
			}
			// Small delay to check for any last-moment additions
			time.Sleep(10 * time.Millisecond)
		} else if hasRunning {
			// Tasks are still running, wait a bit before checking again
			time.Sleep(50 * time.Millisecond)
			stableIterations = 0
		} else {
			// Tasks are pending but not running yet
			time.Sleep(10 * time.Millisecond)
			stableIterations = 0
		}
	}

	// Now get the final results
	_, err := g.GetResults()
	if err != nil {
		result.Error = err
		return result
	}

	result.Status = g.Status()
	result.Duration = g.Duration()

	// For plain render mode, force a final render
	if g.manager != nil && g.manager.noProgress.Load() && !g.manager.noRender.Load() {
		g.manager.PlainRender()
	}

	return result
}

// Cancel cancels all items in the group
func (g *Group) Cancel() {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.cancel != nil {
		g.cancel()
	}

	// Cancel all child items
	for _, item := range g.Items {
		item.GetTask().Cancel()
	}
}

// Duration returns the total duration from first start to last completion
func (g *TypedGroup[T]) Duration() time.Duration {
	if g.startTime.IsZero() {
		return 0
	}

	// Find the latest end time among all items
	var latestEnd time.Time
	allCompleted := true

	for _, item := range g.Items {
		status := item.GetTask().Status()
		if status == StatusPending || status == StatusRunning {
			allCompleted = false
			break
		}

		itemDuration := item.GetTask().Duration()
		if itemDuration > 0 {
			if !item.GetTask().endTime.IsZero() && item.GetTask().endTime.After(latestEnd) {
				latestEnd = item.GetTask().endTime
			}
		}
	}

	if !allCompleted {
		return time.Since(g.startTime)
	}

	if latestEnd.IsZero() {
		return time.Since(g.startTime)
	}

	return latestEnd.Sub(g.startTime)
}

// IsGroup returns true for Group
func (g *Group) IsGroup() bool {
	return true
}

// IsGroup returns true for Group
func (g TypedGroup[T]) IsGroup() bool {
	return true
}
