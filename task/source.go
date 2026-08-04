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

func snapshotsWithSource(ctx context.Context, source RunSource, ids ...string) []TaskSnapshot {
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
		snapshots, err := source.Snapshot(ctx, id)
		if err == nil {
			live = append(live, snapshots...)
		}
	}
	return live
}
