package task

import (
	"github.com/flanksource/commons/text"
)

type LogEntry struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// TaskSnapshot is a JSON-serializable snapshot of a task or group's current state.
type TaskSnapshot struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Type      string     `json:"type"`            // "task" or "group"
	Group     string     `json:"group,omitempty"` // parent group name
	Status    string     `json:"status"`
	Duration  string     `json:"duration,omitempty"`
	Error     string     `json:"error,omitempty"`
	Message   string     `json:"message,omitempty"`   // latest log line
	Logs      []LogEntry `json:"logs,omitempty"`      // all log entries
	Total     int        `json:"total,omitempty"`     // group: total child tasks
	Completed int        `json:"completed,omitempty"` // group: completed tasks
	Failed    int        `json:"failed,omitempty"`    // group: failed tasks
	Running   int        `json:"running,omitempty"`   // group: running tasks
}

// SnapshotTask creates a TaskSnapshot from a Task.
func SnapshotTask(t *Task, groupName string) TaskSnapshot {
	snap := TaskSnapshot{
		ID:     t.Name(),
		Name:   t.Name(),
		Type:   "task",
		Group:  groupName,
		Status: string(t.Status()),
	}
	if d := t.Duration(); d > 0 {
		snap.Duration = text.HumanizeDuration(d)
	}
	if err := t.Error(); err != nil {
		snap.Error = err.Error()
	}
	if bl := t.getBufferedLogger(); bl != nil {
		if entries := bl.GetLogs(); len(entries) > 0 {
			snap.Message = entries[len(entries)-1].Message
			for _, e := range entries {
				snap.Logs = append(snap.Logs, LogEntry{
					Level:   e.Level.String(),
					Message: e.Message,
				})
			}
		}
	}
	return snap
}

// SnapshotGroup creates a TaskSnapshot from a Group with aggregate child stats.
func SnapshotGroup(g *Group) TaskSnapshot {
	snap := TaskSnapshot{
		ID:     g.Name(),
		Name:   g.Name(),
		Type:   "group",
		Status: string(g.Status()),
	}
	g.mu.RLock()
	items := g.Items
	g.mu.RUnlock()

	for _, item := range items {
		t := item.GetTask()
		snap.Total++
		switch t.Status() {
		case StatusSuccess, StatusPASS, StatusSKIP:
			snap.Completed++
		case StatusFailed, StatusFAIL, StatusERR:
			snap.Failed++
		case StatusRunning:
			snap.Running++
		case StatusPending, StatusWarning, StatusCancelled:
			// counted in Total but not in other buckets
		}
	}
	return snap
}

// SnapshotAll returns snapshots for all groups and their tasks.
// If taskIDs is non-empty, only groups whose name matches are included.
func SnapshotAll(taskIDs ...string) []TaskSnapshot {
	if global == nil {
		return nil
	}

	filter := make(map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		filter[id] = true
	}

	var snapshots []TaskSnapshot

	global.mu.RLock()
	groups := make([]*Group, len(global.groups))
	copy(groups, global.groups)
	global.mu.RUnlock()

	for _, g := range groups {
		if len(filter) > 0 && !filter[g.Name()] {
			continue
		}
		snapshots = append(snapshots, SnapshotGroup(g))

		g.mu.RLock()
		items := g.Items
		g.mu.RUnlock()

		for _, item := range items {
			snapshots = append(snapshots, SnapshotTask(item.GetTask(), g.Name()))
		}
	}

	return snapshots
}
