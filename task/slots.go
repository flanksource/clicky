package task

import (
	"fmt"
	"time"
)

// WithSlots sets the concurrency capacity reserved by a task. Ordinary tasks
// use one slot. Supervisors use zero while waiting for their child tasks.
func WithSlots(slots int) Option {
	if slots < 0 {
		panic("task: slots must be non-negative")
	}
	return func(t *Task) { t.slots = slots }
}

func (tm *Manager) reserveSlots(t *Task) (bool, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.maxConcurrent > 0 && t.slots > tm.maxConcurrent {
		return false, fmt.Errorf("task %q requests %d slots, global capacity is %d", t.name, t.slots, tm.maxConcurrent)
	}
	if t.parent != nil && t.parent.concurrency > 0 && t.slots > t.parent.concurrency {
		return false, fmt.Errorf("task %q requests %d slots, group capacity is %d", t.name, t.slots, t.parent.concurrency)
	}
	if tm.maxConcurrent > 0 && tm.slotsUsed+t.slots > tm.maxConcurrent {
		return false, nil
	}
	tm.slotsUsed += t.slots
	return true, nil
}

func (tm *Manager) releaseSlots(t *Task) {
	tm.mu.Lock()
	tm.slotsUsed -= t.slots
	tm.mu.Unlock()
}

// Dispatch supervisors independently because all ordinary workers may already
// be occupied by children waiting for another supervisor.
func (tm *Manager) runSupervisor(t *Task) {
	w := &worker{manager: tm}
	w.inflight.Store(t)
	tm.supervisors.Store(t, w)
	defer tm.supervisors.Delete(t)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for !retiredBeforeStart(t) && !w.checkDependencies(t) {
		select {
		case <-t.ctx.Done():
			t.Cancel()
			t.markCompleted()
			t.signalDone()
			tm.releaseIdentity(t)
			observeGroupTerminal(t.parent)
			return
		case <-ticker.C:
		}
	}
	w.finishTask(t, false)
}
