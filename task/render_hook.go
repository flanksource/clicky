package task

import "github.com/flanksource/clicky/api"

// LiveRenderer replaces the default task-tree rendering with caller-supplied
// content while keeping clicky's terminal ownership: the render loop still owns
// the TTY, ClearLines line accounting, and the logger serializer that prevents
// concurrent log lines from corrupting the in-place frame. Only the rendered
// content (an api.Text) is swapped — a caller that wants its own block (a status
// table, a custom dashboard) renders it through clicky instead of hand-rolling
// ANSI redraws that collide with logger output.
//
// Install with SetLiveRenderer; remove by passing nil. Both methods receive a
// snapshot of the current tasks and must not mutate them.
type LiveRenderer interface {
	// RenderLive returns the content for one live frame — a 250ms interactive
	// tick or a single non-interactive (PlainRender) flush.
	RenderLive(tasks []*Task) api.Text
	// RenderFinal returns the content drawn once after all tasks complete.
	RenderFinal(tasks []*Task) api.Text
}

// SetLiveRenderer installs a custom renderer on the global manager, or removes
// it when r is nil. It is process-global like SetNoRender; install it for the
// duration of one command and restore (SetLiveRenderer(nil)) afterwards.
func SetLiveRenderer(r LiveRenderer) {
	global.setLiveRenderer(r)
}

func (tm *Manager) setLiveRenderer(r LiveRenderer) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.liveRenderer = r
}

func (tm *Manager) getLiveRenderer() LiveRenderer {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.liveRenderer
}

// snapshotTasks returns a lock-free copy of the manager's tasks for rendering.
func (tm *Manager) snapshotTasks() []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	snapshot := make([]*Task, len(tm.tasks))
	copy(snapshot, tm.tasks)
	return snapshot
}
