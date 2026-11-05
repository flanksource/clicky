package task

import (
	"fmt"
	"time"

	"github.com/flanksource/clicky/prompt"
)

// SnapshotStreamLimit bounds each stdout/stderr tail exposed through task
// snapshots. ExecResult remains complete; this limit applies only to task views
// and persisted snapshots.
const SnapshotStreamLimit = 1 << 20

// OutputSnapshot is the live output projected by an executing task. Stdout and
// Stderr are tails: a provider backed by a bounded buffer reports in
// StdoutOffset/StderrOffset how many bytes it has already discarded, so a
// consumer streaming the output can tell an append from a rollover rather than
// inferring it by comparing the text it holds.
type OutputSnapshot struct {
	Stdout       string `json:"stdout,omitempty"`
	Stderr       string `json:"stderr,omitempty"`
	StdoutOffset int64  `json:"stdoutOffset,omitempty"`
	StderrOffset int64  `json:"stderrOffset,omitempty"`
}

// LogEntry is one retained task log line. Seq is its position in the task's
// whole log history, so a stream can send only the entries a client lacks.
type LogEntry struct {
	Seq     uint64 `json:"seq"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Time    string `json:"time"` // RFC3339Nano
}

// TaskSnapshot is a JSON-serializable snapshot of a task or group's current
// state. It carries timestamps only — never an elapsed value, which would
// change on every read of a running task; Duration derives it on demand.
type TaskSnapshot struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Type    string        `json:"type"`            // "task" or "group"
	Group   string        `json:"group,omitempty"` // parent group name
	Status  string        `json:"status"`
	Error   string        `json:"error,omitempty"`
	Retry   *RetryRequest `json:"retry,omitempty"`
	Message string        `json:"message,omitempty"` // latest log line
	Logs    []LogEntry    `json:"logs,omitempty"`    // retained log entries, oldest first
	// LogsClearedThrough is the Seq of the last entry dropped by a log clear (0
	// when the log was never cleared): a client holding entries at or below it
	// holds entries that are gone.
	LogsClearedThrough uint64        `json:"logsClearedThrough,omitempty"`
	Total              int           `json:"total,omitempty"`     // group: total child tasks
	Completed          int           `json:"completed,omitempty"` // group: completed tasks
	Failed             int           `json:"failed,omitempty"`    // group: failed tasks
	Running            int           `json:"running,omitempty"`   // group: running tasks
	Canceled           int           `json:"canceled,omitempty"`
	Work               *WorkProgress `json:"work,omitempty"`

	// Per-task fields (type == "task"). Description is the live stage label set
	// via Task.SetDescription; Progress/MaxValue mirror Task.SetProgress so the UI
	// can render an x/y count and percent for a running task. MaxValue 0 means the
	// task has no bounded progress.
	Description     string           `json:"description,omitempty"`
	Progress        int              `json:"progress,omitempty"`
	MaxValue        int              `json:"maxValue,omitempty"`
	ProgressMetrics *ProgressMetrics `json:"progressMetrics,omitempty"`

	// Registry metadata (additive). For a group these describe the run itself;
	// for a task GroupID links it to its parent run so the SSE/JSON clients can
	// key on a stable id rather than the human-facing name.
	GroupID         string            `json:"groupId,omitempty"`
	ParentGroupID   string            `json:"parentGroupId,omitempty"`
	DependencyIDs   []string          `json:"dependencyIds,omitempty"`
	Kind            string            `json:"kind,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Owner           string            `json:"owner,omitempty"`
	StartedAt       string            `json:"startedAt,omitempty"`  // RFC3339
	FinishedAt      string            `json:"finishedAt,omitempty"` // RFC3339
	Href            string            `json:"href,omitempty"`
	Controls        []ControlAction   `json:"controls,omitempty"`
	Stdout          string            `json:"stdout,omitempty"`
	Stderr          string            `json:"stderr,omitempty"`
	StdoutOffset    int64             `json:"stdoutOffset,omitempty"`
	StderrOffset    int64             `json:"stderrOffset,omitempty"`
	StdoutTruncated bool              `json:"stdoutTruncated,omitempty"`
	StderrTruncated bool              `json:"stderrTruncated,omitempty"`
	Details         any               `json:"details,omitempty"`
	// Prompts are the questions this running task is blocked on (see prompt.TaskLabel).
	Prompts []prompt.PromptSnapshot `json:"prompts,omitempty"`
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
	t.mu.Lock()
	if t.retryRequest != nil && t.status == StatusRunning {
		request := *t.retryRequest
		snap.Retry = &request
	}
	dependencies := append([]*Task(nil), t.dependencies...)
	t.mu.Unlock()
	for _, dependency := range dependencies {
		snap.DependencyIDs = append(snap.DependencyIDs, dependency.ID())
	}
	if group != nil {
		snap.Group = group.Name()
		snap.GroupID = group.ID()
	}
	if started := t.StartTime(); !started.IsZero() {
		snap.StartedAt = started.UTC().Format(time.RFC3339Nano)
	}
	if finished := t.EndTime(); !finished.IsZero() {
		snap.FinishedAt = finished.UTC().Format(time.RFC3339Nano)
	}
	if err := t.Error(); err != nil {
		snap.Error = err.Error()
	}
	t.snapshotProgress(&snap)
	if bl := t.getBufferedLogger(); bl != nil {
		snap.LogsClearedThrough = bl.ClearedThrough()
		if entries := bl.GetLogs(); len(entries) > 0 {
			snap.Message = entries[len(entries)-1].Message
			for _, e := range entries {
				snap.Logs = append(snap.Logs, LogEntry{
					Seq:     e.Seq,
					Level:   e.Level.String(),
					Message: e.Message,
					Time:    e.Time.UTC().Format(time.RFC3339Nano),
				})
			}
		}
	}
	output := t.snapshotOutput()
	snap.Stdout, snap.StdoutOffset, snap.StdoutTruncated = streamTail(output.Stdout, output.StdoutOffset)
	snap.Stderr, snap.StderrOffset, snap.StderrTruncated = streamTail(output.Stderr, output.StderrOffset)
	snap.Details = t.snapshotDetails()
	snap.Prompts = pendingPrompts(t)
	t.mu.Lock()
	controller := t.controller
	t.mu.Unlock()
	snap.Controls = controllerActions(controller)
	return snap
}

// Duration is how long the task or group has run: finishedAt-startedAt once it
// finished, the time since startedAt while it runs, and 0 before it starts.
func (s TaskSnapshot) Duration() time.Duration { return elapsed(s.StartedAt, s.FinishedAt) }

// Duration is how long the run has taken so far; see TaskSnapshot.Duration.
func (m RunMeta) Duration() time.Duration { return elapsed(m.StartedAt, m.FinishedAt) }

func elapsed(startedAt, finishedAt string) time.Duration {
	if startedAt == "" {
		return 0
	}
	started := parseSnapshotTime("startedAt", startedAt)
	if finishedAt == "" {
		return time.Since(started)
	}
	return parseSnapshotTime("finishedAt", finishedAt).Sub(started)
}

// parseSnapshotTime panics on a malformed timestamp: snapshots are written with
// time.RFC3339Nano, so anything else is a corrupted snapshot, not input.
func parseSnapshotTime(field, value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		panic(fmt.Sprintf("task: snapshot %s %q is not RFC3339: %v", field, value, err))
	}
	return parsed
}

// streamTail bounds a stream view to SnapshotStreamLimit, advancing offset by
// whatever it drops so the returned offset still names where the first byte
// sits in the whole stream. Truncation is exactly "something was dropped",
// whether by this cap or by the bounded buffer that produced value.
func streamTail(value string, offset int64) (string, int64, bool) {
	if len(value) > SnapshotStreamLimit {
		dropped := len(value) - SnapshotStreamLimit
		value, offset = value[dropped:], offset+int64(dropped)
	}
	return value, offset, offset > 0
}

// SnapshotGroup creates a TaskSnapshot from a Group with aggregate child stats.
// The snapshot ID stays the group NAME for backward compatibility with
// name-keyed stream consumers; the stable id is carried separately in GroupID. Observing a terminal status here records finishedAt lazily.
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
		Href:    md.Href,
	}
	snap.Controls = controllerActions(controllerForGroup(g))
	if g.parent != nil {
		snap.ParentGroupID = g.parent.ID()
	}
	snap.Details = g.snapshotDetails()
	snap.Work = g.snapshotWork()
	if started := g.StartedAt(); !started.IsZero() {
		snap.StartedAt = started.UTC().Format(time.RFC3339Nano)
	}
	if finished := g.FinishedAt(); !finished.IsZero() {
		snap.FinishedAt = finished.UTC().Format(time.RFC3339Nano)
	}
	items := g.allTasks()

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
		case StatusCancelled:
			snap.Canceled++
		case StatusPending, StatusWarning:
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
	for _, child := range g.childGroups() {
		snaps = append(snaps, snapshotGroupWithTasks(child)...)
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
		includedByParent := false
		for parent := g.parent; parent != nil; parent = parent.parent {
			if len(filter) == 0 || filter[parent.Name()] || filter[parent.ID()] {
				includedByParent = true
				break
			}
		}
		if includedByParent {
			continue
		}
		snapshots = append(snapshots, snapshotGroupWithTasks(g)...)
	}

	return snapshots
}
