package task

import (
	"time"

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

	// Per-task fields (type == "task"). Description is the live stage label set
	// via Task.SetDescription; Progress/MaxValue mirror Task.SetProgress so the UI
	// can render an x/y count and percent for a running task. MaxValue 0 means the
	// task has no bounded progress.
	Description string `json:"description,omitempty"`
	Progress    int    `json:"progress,omitempty"`
	MaxValue    int    `json:"maxValue,omitempty"`

	// Registry metadata (additive). For a group these describe the run itself;
	// for a task GroupID links it to its parent run so the SSE/JSON clients can
	// key on a stable id rather than the human-facing name.
	GroupID    string            `json:"groupId,omitempty"`
	Kind       string            `json:"kind,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Owner      string            `json:"owner,omitempty"`
	StartedAt  string            `json:"startedAt,omitempty"`  // RFC3339
	FinishedAt string            `json:"finishedAt,omitempty"` // RFC3339
}

// SnapshotTask creates a TaskSnapshot from a Task. group is the parent group, or
// nil for an ungrouped task; its name and id are recorded on the snapshot.
func SnapshotTask(t *Task, group *Group) TaskSnapshot {
	snap := TaskSnapshot{
		ID:     t.ID(),
		Name:   t.Name(),
		Type:   "task",
		Status: string(t.Status()),
	}
	if group != nil {
		snap.Group = group.Name()
		snap.GroupID = group.ID()
	}
	if d := t.Duration(); d > 0 {
		snap.Duration = text.HumanizeDuration(d)
	}
	if err := t.Error(); err != nil {
		snap.Error = err.Error()
	}
	snap.Description = t.Description()
	if value, max := t.Progress(); max > 0 {
		snap.Progress = value
		snap.MaxValue = max
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
// The snapshot ID stays the group NAME for backward compatibility with the
// name-keyed Preact UI and JSONHandler; the stable id is carried separately in
// GroupID. Observing a terminal status here records finishedAt lazily.
func SnapshotGroup(g *Group) TaskSnapshot {
	status := g.Status()
	g.observeTerminal(status, time.Now())
	md := g.Metadata()
	snap := TaskSnapshot{
		ID:      g.Name(),
		Name:    g.Name(),
		Type:    "group",
		Status:  string(status),
		GroupID: g.ID(),
		Kind:    md.Kind,
		Labels:  md.Labels,
		Owner:   md.Owner,
	}
	if started := g.StartedAt(); !started.IsZero() {
		snap.StartedAt = started.UTC().Format(time.RFC3339)
	}
	if finished := g.FinishedAt(); !finished.IsZero() {
		snap.FinishedAt = finished.UTC().Format(time.RFC3339)
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

// snapshotGroupWithTasks returns the group snapshot followed by its child task
// snapshots. The group's own lock is acquired internally; the caller must NOT
// hold global.mu exclusively (RLock is fine).
func snapshotGroupWithTasks(g *Group) []TaskSnapshot {
	snaps := []TaskSnapshot{SnapshotGroup(g)}
	g.mu.RLock()
	items := g.Items
	g.mu.RUnlock()
	for _, item := range items {
		snaps = append(snaps, SnapshotTask(item.GetTask(), g))
	}
	return snaps
}

// SnapshotAll returns snapshots for all groups and their tasks.
// If taskIDs is non-empty, only groups whose name OR stable id matches are
// included (matching by id lets the registry/SSE drill into one run; matching
// by name preserves the legacy name-keyed callers).
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
		if len(filter) > 0 && !filter[g.Name()] && !filter[g.ID()] {
			continue
		}
		snapshots = append(snapshots, snapshotGroupWithTasks(g)...)
	}

	return snapshots
}
