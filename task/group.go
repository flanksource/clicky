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
	startTime time.Time
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
		currentCount := len(g.Group.Items)
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
		for _, item := range g.Group.Items {
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

	// Force a final render to ensure all completed tasks are displayed
	if g.manager != nil {
		g.manager.Render()
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
