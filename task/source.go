package task

import (
	"context"
	"sort"

	"github.com/flanksource/clicky/metrics"
)

// RunSource exposes task runs owned by another process or durable store.
// In-memory runs always win when an ID exists in both sources.
type RunSource interface {
	Runs(context.Context, RunFilter) ([]RunMeta, error)
	Snapshot(context.Context, string) ([]TaskSnapshot, error)
	Control(context.Context, string, ControlAction) error
}

// RunAuthorizer filters both live and externally stored runs for a caller.
type RunAuthorizer interface {
	AuthorizeRun(context.Context, RunMeta) bool
}

func visibleRuns(ctx context.Context, source RunSource, runs []RunMeta) []RunMeta {
	authorizer, ok := source.(RunAuthorizer)
	if !ok {
		return runs
	}
	visible := make([]RunMeta, 0, len(runs))
	for _, run := range runs {
		if authorizer.AuthorizeRun(ctx, run) {
			visible = append(visible, run)
		}
	}
	return visible
}

func visibleSnapshots(ctx context.Context, source RunSource, snapshots []TaskSnapshot) []TaskSnapshot {
	authorizer, ok := source.(RunAuthorizer)
	if !ok {
		return snapshots
	}
	allowed := make(map[string]bool)
	for _, snapshot := range snapshots {
		if snapshot.Type == "group" {
			allowed[snapshot.GroupID] = authorizer.AuthorizeRun(ctx, RunMetaFromSnapshot(snapshot))
		}
	}
	visible := make([]TaskSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if allowed[snapshot.GroupID] {
			visible = append(visible, snapshot)
		}
	}
	return visible
}

func authorizedRun(ctx context.Context, source RunSource, id string) bool {
	if _, ok := source.(RunAuthorizer); !ok {
		return true
	}
	snapshots := SnapshotByID(id)
	if len(snapshots) == 0 && source != nil {
		var err error
		snapshots, err = source.Snapshot(ctx, id)
		if err != nil {
			return false
		}
	}
	return len(visibleSnapshots(ctx, source, snapshots)) != 0
}

// TaskControlSource is implemented by run sources that can route lifecycle
// controls to child tasks owned outside this process.
type TaskControlSource interface {
	ControlTask(context.Context, string, string, ControlAction) error
}

// MetricSource is optionally implemented by a RunSource that owns live metrics.
type MetricSource interface {
	QueryMetric(context.Context, metrics.QueryRequest) ([]metrics.Point, error)
}

type sourceMetrics struct {
	local    metrics.Timeseries
	external MetricSource
}

func (s sourceMetrics) Record(request metrics.RecordRequest) error { return s.local.Record(request) }

func (s sourceMetrics) Query(request metrics.QueryRequest) ([]metrics.Point, error) {
	points, err := s.local.Query(request)
	if err != nil || len(points) > 0 || s.external == nil {
		return points, err
	}
	return s.external.QueryMetric(context.Background(), request)
}

func mergeRunMetas(live, external []RunMeta) []RunMeta {
	seen := make(map[string]struct{}, len(live)+len(external))
	merged := make([]RunMeta, 0, len(live)+len(external))
	for _, run := range append(live, external...) {
		if _, ok := seen[run.ID]; ok {
			continue
		}
		seen[run.ID] = struct{}{}
		merged = append(merged, run)
	}
	sort.SliceStable(merged, func(i, j int) bool { return merged[i].StartedAt > merged[j].StartedAt })
	return merged
}

// finishedSnapshots remembers the snapshots of external runs whose group has
// reached a terminal status. A finished run never changes, so a stream that
// polls every tick only needs to ask the source for it once.
type finishedSnapshots map[string][]TaskSnapshot

func snapshotsWithSource(ctx context.Context, source RunSource, finished finishedSnapshots, ids ...string) []TaskSnapshot {
	live := SnapshotAll(ids...)
	if source == nil {
		return live
	}
	if len(ids) == 0 {
		runs, err := source.Runs(ctx, RunFilter{})
		if err != nil {
			return live
		}
		for _, run := range runs {
			ids = append(ids, run.ID)
		}
	}
	seen := make(map[string]struct{})
	for _, snapshot := range live {
		if snapshot.Type == "group" {
			seen[snapshot.GroupID] = struct{}{}
		}
	}
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		if snapshots, ok := finished[id]; ok {
			live = append(live, snapshots...)
			continue
		}
		snapshots, err := source.Snapshot(ctx, id)
		if err != nil {
			continue
		}
		if len(snapshots) > 0 && finished != nil && !hasUnfinishedGroup(snapshots) {
			finished[id] = snapshots
		}
		live = append(live, snapshots...)
	}
	return live
}

func hasUnfinishedGroup(snapshots []TaskSnapshot) bool {
	for _, snapshot := range snapshots {
		if snapshot.Type == "group" && (snapshot.Status == string(StatusRunning) || snapshot.Status == string(StatusPending)) {
			return true
		}
	}
	return false
}
