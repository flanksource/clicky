package telltale

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/flanksource/clicky/exec"
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

func (taskProbe) Arm(ctx context.Context, raw json.RawMessage, sink sdk.Sink) (func() error, error) {
	p, err := parseTaskParams(raw)
	if err != nil {
		return nil, err
	}
	queue := newTaskQueue(sink)
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
			queue.push(event.Phase, snapshot, false)
		}
	})
	// The baseline can exceed the SDK's record ring, which only drains once Arm
	// returns, so every record goes through one ordered queue drained after it.
	for _, snapshot := range task.SnapshotAll() {
		if snapshot.Type == "task" && p.matchesName(snapshot.Name) && p.matchesGroup(snapshot.Group) {
			queue.push(taskBaseline, snapshot, true)
		}
	}
	stop := queue.drain(ctx)
	return func() error { release(); stop(); return nil }, nil
}

const (
	// taskQueueLimit bounds live records waiting behind a full ring; the
	// baseline is always queued in full.
	taskQueueLimit = 4096
	taskEmitRetry  = 10 * time.Millisecond
	// taskEmitTimeout is how long one record may wait for room in the ring.
	taskEmitTimeout = 5 * time.Second
)

// taskQueue delivers task records in order, waiting for room in the SDK ring
// instead of dropping a record the moment it is full.
type taskQueue struct {
	sink    sdk.Sink
	mu      sync.Mutex
	records []TaskRecord
	wake    chan struct{}
}

func newTaskQueue(sink sdk.Sink) *taskQueue {
	return &taskQueue{sink: sink, wake: make(chan struct{}, 1)}
}

func (q *taskQueue) push(phase task.TaskPhase, snapshot task.TaskSnapshot, baseline bool) {
	record, err := makeTaskRecord(phase, snapshot)
	if err != nil {
		q.sink.Warn(fmt.Sprintf("task %s: %v", snapshot.ID, err))
		return
	}
	q.mu.Lock()
	full := !baseline && len(q.records) >= taskQueueLimit
	if !full {
		q.records = append(q.records, record)
	}
	q.mu.Unlock()
	if full {
		q.sink.Warn(fmt.Sprintf("task %s: %s record dropped, %d records already queued", snapshot.ID, phase, taskQueueLimit))
		return
	}
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *taskQueue) pop() (TaskRecord, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.records) == 0 {
		return TaskRecord{}, false
	}
	record := q.records[0]
	q.records = q.records[1:]
	return record, true
}

// drain emits queued records until the returned stop is called or ctx ends.
func (q *taskQueue) drain(ctx context.Context) (stop func()) {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			record, ok := q.pop()
			if !ok {
				select {
				case <-ctx.Done():
					return
				case <-q.wake:
					continue
				}
			}
			if !q.emit(ctx, record) {
				return
			}
		}
	}()
	return func() { cancel(); <-done }
}

// emit retries a record while the ring is full; false means ctx ended.
func (q *taskQueue) emit(ctx context.Context, record TaskRecord) bool {
	deadline := time.Now().Add(taskEmitTimeout)
	for !q.sink.TryEmit(record) {
		if time.Now().After(deadline) {
			q.sink.Warn(fmt.Sprintf("task %s: %s record dropped after %s waiting for the record ring", record.ID, record.Phase, taskEmitTimeout))
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(taskEmitRetry):
		}
	}
	return true
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

func makeTaskRecord(phase task.TaskPhase, snapshot task.TaskSnapshot) (TaskRecord, error) {
	snapshot.Logs = nil
	snapshot.Stdout, snapshot.Stderr = "", ""
	snapshot.Message = sdk.Mask(snapshot.Message)
	snapshot.Error = sdk.Mask(snapshot.Error)
	details, err := maskTaskDetails(snapshot.Details)
	if err != nil {
		return TaskRecord{}, err
	}
	snapshot.Details = details
	return TaskRecord{Phase: phase, TaskSnapshot: snapshot}, nil
}

// maskTaskDetails masks an exec-backed task's command line with argument-aware
// masking, then masks secret keys and values in whatever details remain.
func maskTaskDetails(details any) (any, error) {
	switch d := details.(type) {
	case nil:
		return nil, nil
	case exec.ExecTaskDetails:
		d.Args, d.Command = maskArgv(d.Command, d.Args), maskCommand(d.Command)
		details = d
	case exec.ProcessDetails:
		d.Args, d.Command = maskArgv(d.Command, d.Args), maskCommand(d.Command)
		details = d
	}
	data, err := json.Marshal(details)
	if err != nil {
		return nil, fmt.Errorf("encode task details: %w", err)
	}
	return json.RawMessage(sdk.Mask(string(data))), nil
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
