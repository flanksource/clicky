package telltale

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/flanksource/clicky/task"
	sdk "github.com/flanksource/telltale/sdk"
	"github.com/flanksource/telltale/sdk/protocol"
)

// TaskRecord is a clicky task snapshot at a status transition, a throttled
// progress update, or (Phase "baseline") a task already present when the
// probe was armed. Logs and captured output are left to the logs and exec
// kinds; only the latest log line survives as Message.
type TaskRecord struct {
	Phase task.TaskPhase `json:"phase"`
	task.TaskSnapshot
}

const taskBaseline task.TaskPhase = "baseline"

type taskParams struct {
	Name               string `json:"name"`
	Group              string `json:"group"`
	ProgressIntervalMs int    `json:"progressIntervalMs"`
}

type taskProbe struct{}

func (taskProbe) Spec() protocol.ProbeSpec { return protocol.ProbeSpec{Name: "task", Title: "Tasks"} }

func (taskProbe) Arm(_ context.Context, raw json.RawMessage, sink sdk.Sink) (func() error, error) {
	p, err := parseTaskParams(raw)
	if err != nil {
		return nil, err
	}
	throttle := progressThrottle{interval: time.Duration(p.ProgressIntervalMs) * time.Millisecond, last: map[string]time.Time{}}
	release := task.ObserveTasks(func(event task.TaskEvent) {
		if !p.matchesName(event.Task.Name()) {
			return
		}
		switch {
		case event.Phase == task.TaskProgressed && !throttle.allow(event.Task.ID(), time.Now()):
			return
		case event.Status == task.StatusSuccess, event.Status == task.StatusFailed,
			event.Status == task.StatusCancelled, event.Status == task.StatusWarning:
			throttle.forget(event.Task.ID())
		}
		snapshot := event.Snapshot()
		if p.matchesGroup(snapshot.Group) {
			sink.TryEmit(makeTaskRecord(event.Phase, snapshot))
		}
	})
	for _, snapshot := range task.SnapshotAll() {
		if snapshot.Type == "task" && p.matchesName(snapshot.Name) && p.matchesGroup(snapshot.Group) {
			sink.TryEmit(makeTaskRecord(taskBaseline, snapshot))
		}
	}
	return func() error { release(); return nil }, nil
}

func parseTaskParams(raw json.RawMessage) (taskParams, error) {
	p := taskParams{ProgressIntervalMs: 1000}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return p, fmt.Errorf("task params: %w", err)
		}
	}
	if p.Name == "" {
		p.Name = "*"
	}
	if p.Group == "" {
		p.Group = "*"
	}
	for _, glob := range []string{p.Name, p.Group} {
		if _, err := filepath.Match(glob, ""); err != nil {
			return p, fmt.Errorf("task glob %q: %w", glob, err)
		}
	}
	if p.ProgressIntervalMs < 0 {
		return p, fmt.Errorf("task progressIntervalMs must not be negative")
	}
	return p, nil
}

func (p taskParams) matchesName(name string) bool {
	match, _ := filepath.Match(p.Name, name)
	return match
}

func (p taskParams) matchesGroup(group string) bool {
	match, _ := filepath.Match(p.Group, group)
	return match
}

func makeTaskRecord(phase task.TaskPhase, snapshot task.TaskSnapshot) TaskRecord {
	snapshot.Logs = nil
	snapshot.Stdout, snapshot.Stderr = "", ""
	snapshot.Message = sdk.Mask(snapshot.Message)
	snapshot.Error = sdk.Mask(snapshot.Error)
	return TaskRecord{Phase: phase, TaskSnapshot: snapshot}
}

type progressThrottle struct {
	mu       sync.Mutex
	interval time.Duration
	last     map[string]time.Time
}

func (t *progressThrottle) allow(id string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if previous, ok := t.last[id]; ok && now.Sub(previous) < t.interval {
		return false
	}
	t.last[id] = now
	return true
}

func (t *progressThrottle) forget(id string) {
	t.mu.Lock()
	delete(t.last, id)
	t.mu.Unlock()
}
