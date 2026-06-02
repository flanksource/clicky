package task

import (
	"sort"
	"strings"
	"time"
)

// runRetention is how long a finished run is kept queryable before GC removes
// it from the global manager. Live runs are never GC'd.
const runRetention = 10 * time.Minute

// OnBeforeGC, if non-nil, is called with each group's full snapshot just before
// GCRuns removes it from the in-memory manager. The callback receives the
// group's stable ID and the full snapshot slice (group + child tasks). It is
// called while global.mu is held, so it must not call back into the task
// package.
var OnBeforeGC func(groupID string, snapshots []TaskSnapshot)

// RunMeta is the listing summary for one run (task group): identity, metadata,
// status, timing, and child-task counts. It is what the generic task-manager
// list view renders; drill-down uses SnapshotByID.
type RunMeta struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Kind       string            `json:"kind,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Owner      string            `json:"owner,omitempty"`
	Status     string            `json:"status"`
	StartedAt  string            `json:"startedAt,omitempty"`  // RFC3339
	FinishedAt string            `json:"finishedAt,omitempty"` // RFC3339
	Total      int               `json:"total"`
	Completed  int               `json:"completed"`
	Failed     int               `json:"failed"`
	Running    int               `json:"running"`
}

// RunFilter narrows the runs returned by Runs. Empty fields match everything.
type RunFilter struct {
	Kind   string
	Status string
	Labels map[string]string // every entry must match
}

func (f RunFilter) Matches(m RunMeta) bool {
	if f.Kind != "" && m.Kind != f.Kind {
		return false
	}
	if f.Status != "" && m.Status != f.Status {
		return false
	}
	for k, v := range f.Labels {
		if m.Labels[k] != v {
			return false
		}
	}
	return true
}

// RunMetaFromSnapshot lifts the group-level fields of a group snapshot into a
// RunMeta. The snapshot's ID is the group name; GroupID carries the stable id.
func RunMetaFromSnapshot(snap TaskSnapshot) RunMeta {
	return RunMeta{
		ID:         snap.GroupID,
		Name:       snap.Name,
		Kind:       snap.Kind,
		Labels:     snap.Labels,
		Owner:      snap.Owner,
		Status:     snap.Status,
		StartedAt:  snap.StartedAt,
		FinishedAt: snap.FinishedAt,
		Total:      snap.Total,
		Completed:  snap.Completed,
		Failed:     snap.Failed,
		Running:    snap.Running,
	}
}

// Runs returns one RunMeta per registered group, newest-first, optionally
// narrowed by filter. It runs GC first so stale finished runs drop out.
func Runs(filter RunFilter) []RunMeta {
	GCRuns()
	return RunsRaw(filter)
}

// RunsRaw is like Runs but does NOT trigger GC first. Callers that manage
// their own GC timing (e.g. an L2-backed wrapper that needs to snapshot
// before GC) use this to avoid double-GC.
func RunsRaw(filter RunFilter) []RunMeta {
	if global == nil {
		return nil
	}
	global.mu.RLock()
	groups := make([]*Group, len(global.groups))
	copy(groups, global.groups)
	global.mu.RUnlock()

	out := make([]RunMeta, 0, len(groups))
	for _, g := range groups {
		meta := RunMetaFromSnapshot(SnapshotGroup(g))
		if filter.Matches(meta) {
			out = append(out, meta)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt > out[j].StartedAt })
	return out
}

// SnapshotByID returns the group + task snapshots for the run with the given
// stable id (not name). Returns nil when no such run exists.
func SnapshotByID(id string) []TaskSnapshot {
	return SnapshotAll(id)
}

// GCRuns removes finished runs older than runRetention from the global manager.
// A run is "finished" once it has a non-zero FinishedAt (recorded the first time
// it is observed terminal). Live runs are retained regardless of age. If
// OnBeforeGC is set, each evicted group's full snapshot is passed to it before
// removal.
func GCRuns() {
	if global == nil {
		return
	}
	now := time.Now()
	global.mu.Lock()
	defer global.mu.Unlock()
	kept := global.groups[:0]
	for _, g := range global.groups {
		g.observeTerminal(g.Status(), now)
		finished := g.FinishedAt()
		if !finished.IsZero() && now.Sub(finished) > runRetention {
			if OnBeforeGC != nil {
				OnBeforeGC(g.ID(), snapshotGroupWithTasks(g))
			}
			continue
		}
		kept = append(kept, g)
	}
	global.groups = kept
}

// parseLabelFilter parses repeated "k=v" query values into a label match map.
func parseLabelFilter(values []string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	labels := make(map[string]string, len(values))
	for _, v := range values {
		k, val, ok := strings.Cut(v, "=")
		if !ok {
			continue
		}
		labels[k] = val
	}
	if len(labels) == 0 {
		return nil
	}
	return labels
}
