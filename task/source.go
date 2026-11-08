package task

import (
	"context"
	"fmt"
	"sort"
	"time"

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

// runSnapshots reads the runs one stream follows: the in-memory registry on
// every read, an external source at most once per externalPollInterval. The
// cache is per stream on purpose: a source may scope what it returns to the
// caller in ctx, so sharing its answers across connections could leak runs.
type runSnapshots struct {
	source    RunSource
	ids       []string
	followAll bool
	// finished holds external runs whose group has reached a terminal status.
	// A finished run never changes, so the source is asked for it once.
	finished map[string][]TaskSnapshot
	external []TaskSnapshot
	polledAt time.Time
}

// newRunSnapshots keeps an empty filtered selection distinct from following all runs.
func newRunSnapshots(source RunSource, ids []string, followAll bool) *runSnapshots {
	return &runSnapshots{source: source, ids: ids, followAll: followAll, finished: map[string][]TaskSnapshot{}}
}

// read returns the live runs plus the external ones as of the last refresh,
// refreshing those first when now is a poll interval past the last one.
func (s *runSnapshots) read(ctx context.Context, now time.Time) ([]TaskSnapshot, error) {
	if !s.followAll && len(s.ids) == 0 {
		return nil, nil
	}
	live := SnapshotAll(s.ids...)
	if s.source == nil {
		return live, nil
	}
	held := heldRuns(live)
	if s.polledAt.IsZero() || now.Sub(s.polledAt) >= externalPollInterval {
		external, err := s.readExternal(ctx, held)
		if err != nil {
			return nil, err
		}
		s.external, s.polledAt = external, now
	}
	for _, snapshot := range s.external {
		if !held[snapshot.GroupID] {
			live = append(live, snapshot)
		}
	}
	return live, nil
}

// readExternal refreshes selected remote runs, reusing terminal snapshots and
// leaving locally held runs to the in-memory registry.
func (s *runSnapshots) readExternal(ctx context.Context, held map[string]bool) ([]TaskSnapshot, error) {
	ids := s.ids
	if s.followAll {
		runs, err := s.source.Runs(ctx, RunFilter{})
		if err != nil {
			return nil, fmt.Errorf("list external task runs: %w", err)
		}
		for _, run := range runs {
			ids = append(ids, run.ID)
		}
	}
	var external []TaskSnapshot
	for _, id := range ids {
		if held[id] {
			continue
		}
		if snapshots, ok := s.finished[id]; ok {
			external = append(external, snapshots...)
			continue
		}
		snapshots, err := s.source.Snapshot(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("load external task run %q: %w", id, err)
		}
		if len(snapshots) > 0 && !hasUnfinishedGroup(snapshots) {
			s.finished[id] = snapshots
		}
		external = append(external, snapshots...)
	}
	return external, nil
}

// heldRuns names the runs this process holds, by stable id and by name: a
// stream may follow a run by either, and neither is the source's to answer.
func heldRuns(snapshots []TaskSnapshot) map[string]bool {
	held := map[string]bool{}
	for _, snapshot := range snapshots {
		if snapshot.Type == "group" {
			held[snapshot.GroupID] = true
			held[snapshot.ID] = true
		}
	}
	return held
}

func hasUnfinishedGroup(snapshots []TaskSnapshot) bool {
	for _, snapshot := range snapshots {
		if snapshot.Type == "group" && (snapshot.Status == string(StatusRunning) || snapshot.Status == string(StatusPending)) {
			return true
		}
	}
	return false
}
