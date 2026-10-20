package task

import (
	"fmt"
	"time"
)

// PlanStep registers an externally-driven phase without taking a worker slot.
// Its owner calls Start before the operation and Finish with its outcome.
func (g *Group) PlanStep(name string, opts ...Option) *Step {
	t := g.manager.newTask(name, append([]Option{WithGroup(g)}, opts...)...)
	t.mu.Lock()
	t.startTime, t.enqueuedAt, t.maxValue = time.Time{}, time.Time{}, 0
	t.mu.Unlock()
	attachTaskableToGroup(t, t)
	g.manager.mu.Lock()
	g.manager.tasks = append(g.manager.tasks, t)
	g.manager.mu.Unlock()
	return &Step{task: t}
}

func (s *Step) Start() error {
	t := s.task
	if err := t.Context().Err(); err != nil {
		return err
	}
	for _, dependency := range t.dependencies {
		switch dependency.Status() {
		case StatusSuccess, StatusPASS, StatusSKIP:
		default:
			return fmt.Errorf("%s is waiting for %s (%s)", t.Name(), dependency.Name(), dependency.Status())
		}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.status != StatusPending || t.completed.Load() {
		return fmt.Errorf("step %s cannot start from %s", t.name, t.status)
	}
	t.status, t.startTime, t.enqueuedAt = StatusRunning, time.Now(), time.Now()
	t.dirty.Store(true)
	return nil
}
