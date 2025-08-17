package task

import (
	"context"
	"sync"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
)

// Group represents a group of tasks that can be managed collectively
type Group struct {
	name      string
	Items     []Taskable // Can contain Tasks or nested Groups
	status    Status
	startTime time.Time
	endTime   time.Time
	manager   *Manager
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.RWMutex
}

type TaskGroup interface {
	GetTasks() []Taskable
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

	// Create the task using the group's manager
	task := StartTask(name, taskFunc, opts...)

	// Add to the group's items
	g.Group.Items = append(g.Group.Items, task)
	task.parent = g.Group

	// Update start time if this is the first item or it started earlier
	if g.startTime.IsZero() || task.startTime.Before(g.startTime) {
		g.startTime = task.startTime
	}
	return task
}

// GetResults waits for all tasks in the group and returns typed results
func (g TypedGroup[T]) GetResults() (map[TypedTask[T]]T, error) {
	results := make(map[TypedTask[T]]T)
	for _, item := range g.Group.Items {
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
func (g *TypedGroup[T]) WaitFor() *WaitResult {
	result := &WaitResult{}

	_, err := g.GetResults()
	if err != nil {
		result.Error = err
		return result
	}

	// for _, childResult := range items {
	// 	result.TaskCount += childResult.TaskCount
	// 	result.SuccessCount += childResult.SuccessCount
	// 	result.FailureCount += childResult.FailureCount
	// 	result.WarningCount += childResult.WarningCount

	// 	// Keep the first error encountered
	// 	if result.Error == nil && childResult.Error != nil {
	// 		result.Error = childResult.Error
	// 	}
	// }

	result.Status = g.Status()
	result.Duration = g.Duration()

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

	for _, item := range g.Group.Items {
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
