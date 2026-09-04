package task

import (
	"testing"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
)

// withTestGlobal swaps the package global for a fresh, non-rendering test
// manager and restores it on cleanup, so registry tests don't see groups from
// other tests or the real process manager.
func withTestGlobal(t *testing.T) {
	t.Helper()
	original := global
	global = newTestManager(2)
	t.Cleanup(func() {
		global.stopRender()
		global = original
	})
}

// runGroupToCompletion adds one task per name and blocks until the group is
// terminal.
func runGroupToCompletion(t *testing.T, g TypedGroup[any], names ...string) {
	t.Helper()
	for _, name := range names {
		g.Add(name, func(ctx flanksourceContext.Context, tk *Task) (any, error) {
			tk.Success()
			return nil, nil
		})
	}
	deadline := time.After(5 * time.Second)
	for {
		if s := g.Status(); s != StatusRunning && s != StatusPending {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("group %q did not finish; status=%s", g.Name(), g.Status())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestRunsListsGroupsWithMetadataAndDrillsDownByID(t *testing.T) {
	withTestGlobal(t)

	gFix := StartGroup[any]("fix-run",
		WithKind("sql-fix"),
		WithLabels(map[string]string{"database": "OI" + "PA"}),
		WithConcurrency(1),
	)
	gTest := StartGroup[any]("test-run",
		WithKind("test-run"),
		WithConcurrency(1),
	)
	runGroupToCompletion(t, gFix, "rebuild idx_a", "update stats")
	runGroupToCompletion(t, gTest, "step 0")

	runs := Runs(RunFilter{})
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}

	byKind := Runs(RunFilter{Kind: "sql-fix"})
	if len(byKind) != 1 || byKind[0].Name != "fix-run" {
		t.Fatalf("kind filter expected [fix-run], got %+v", byKind)
	}
	if byKind[0].Total != 2 || byKind[0].Completed != 2 {
		t.Fatalf("expected 2/2 completed, got %d/%d", byKind[0].Completed, byKind[0].Total)
	}

	byLabel := Runs(RunFilter{Labels: map[string]string{"database": "OI" + "PA"}})
	if len(byLabel) != 1 || byLabel[0].Kind != "sql-fix" {
		t.Fatalf("label filter expected sql-fix run, got %+v", byLabel)
	}

	// Drill down by stable id returns only that run's group + tasks.
	id := gFix.ID()
	snaps := SnapshotByID(id)
	groups, tasks := 0, 0
	for _, s := range snaps {
		switch s.Type {
		case "group":
			groups++
			if s.GroupID != id {
				t.Fatalf("group snapshot GroupID = %q, want %q", s.GroupID, id)
			}
		case "task":
			tasks++
			if s.GroupID != id {
				t.Fatalf("task snapshot GroupID = %q, want %q", s.GroupID, id)
			}
		}
	}
	if groups != 1 || tasks != 2 {
		t.Fatalf("drill-down expected 1 group + 2 tasks, got %d + %d", groups, tasks)
	}
}

func TestFinishedAtRecordedOnTerminal(t *testing.T) {
	withTestGlobal(t)
	g := StartGroup[any]("done-run", WithConcurrency(1))
	runGroupToCompletion(t, g, "only step")

	// Snapshotting observes terminal and records finishedAt.
	_ = SnapshotGroup(g.Group)
	if g.FinishedAt().IsZero() {
		t.Fatal("expected FinishedAt to be set after terminal snapshot")
	}
}

func TestGCRunsDropsExpiredFinishedRuns(t *testing.T) {
	withTestGlobal(t)
	g := StartGroup[any]("old-run", WithConcurrency(1))
	runGroupToCompletion(t, g, "step")

	// Force an old finishedAt so GC removes it.
	g.mu.Lock()
	g.finishedAt = time.Now().Add(-2 * runRetention)
	g.mu.Unlock()

	GCRuns()
	if got := len(Runs(RunFilter{})); got != 0 {
		t.Fatalf("expected expired run to be GC'd, still have %d", got)
	}
}

func TestGCPersistsSnapshotBeforeEviction(t *testing.T) {
	withTestGlobal(t)
	store := installTestStore(t)

	g := StartGroup[any]("gc-hook-run", WithKind("test"), WithConcurrency(1))
	groupID := g.ID()
	runGroupToCompletion(t, g, "step-a", "step-b")

	// Completing the run persists it once, on the terminal transition. The
	// eviction write is a second, separate one — the last chance to persist a
	// run before the group it would be read from is gone.
	if saved := store.await(t); saved.ID != groupID {
		t.Fatalf("terminal save id = %q, want %q", saved.ID, groupID)
	}

	g.mu.Lock()
	g.finishedAt = time.Now().Add(-2 * runRetention)
	g.mu.Unlock()

	GCRuns()

	saved := store.await(t)
	if saved.ID != groupID {
		t.Fatalf("eviction save id = %q, want %q", saved.ID, groupID)
	}
	if len(saved.Snapshots) != 3 {
		t.Fatalf("expected 3 snapshots (1 group + 2 tasks), got %d", len(saved.Snapshots))
	}
	if saved.Snapshots[0].Type != "group" {
		t.Fatalf("first snapshot should be group, got %q", saved.Snapshots[0].Type)
	}
	if saved.Snapshots[0].Total != 2 {
		t.Fatalf("group snapshot Total = %d, want 2", saved.Snapshots[0].Total)
	}
}

func TestGCDoesNotEvictRunsWithinRetention(t *testing.T) {
	withTestGlobal(t)
	store := installTestStore(t)

	g := StartGroup[any]("live-run", WithConcurrency(1))
	runGroupToCompletion(t, g, "step")
	store.await(t) // the terminal save, which is not what this test is about

	GCRuns()

	if run, ok := store.tryAwait(); ok {
		t.Fatalf("run %q was evicted while inside the retention period", run.ID)
	}
	if got := len(RunsRaw(RunFilter{})); got != 1 {
		t.Fatalf("expected the run to be retained, have %d", got)
	}
}

// A finished run is persisted as soon as it finishes, not ten minutes later when
// GC gets to it — otherwise a restart inside the retention window loses it.
func TestTerminalTransitionPersistsRunImmediately(t *testing.T) {
	withTestGlobal(t)
	store := installTestStore(t)

	g := StartGroup[any]("terminal-run", WithKind("test"), WithConcurrency(1))
	groupID := g.ID()
	runGroupToCompletion(t, g, "only-step")

	saved := store.await(t)
	if saved.ID != groupID {
		t.Fatalf("saved run id = %q, want %q", saved.ID, groupID)
	}
	if saved.Kind != "test" {
		t.Fatalf("saved run kind = %q, want %q", saved.Kind, "test")
	}
	if saved.FinishedAt == "" {
		t.Fatal("saved run has no FinishedAt")
	}
	if got := len(RunsRaw(RunFilter{})); got != 1 {
		t.Fatalf("run should still be live in memory, have %d", got)
	}
}

func TestRunsRawSkipsGC(t *testing.T) {
	withTestGlobal(t)

	g := StartGroup[any]("raw-run", WithConcurrency(1))
	runGroupToCompletion(t, g, "step")

	g.mu.Lock()
	g.finishedAt = time.Now().Add(-2 * runRetention)
	g.mu.Unlock()

	runs := RunsRaw(RunFilter{})
	if len(runs) != 1 {
		t.Fatalf("RunsRaw should return expired runs (no GC), got %d", len(runs))
	}

	runs = Runs(RunFilter{})
	if len(runs) != 0 {
		t.Fatalf("Runs should GC expired runs, got %d", len(runs))
	}
}

func TestSnapshotAllBackwardCompatByName(t *testing.T) {
	withTestGlobal(t)
	g := StartGroup[any]("named-run", WithConcurrency(1))
	runGroupToCompletion(t, g, "step")

	// Legacy name-keyed filtering still works alongside id-keyed.
	byName := SnapshotAll("named-run")
	byID := SnapshotAll(g.ID())
	if len(byName) == 0 || len(byID) == 0 {
		t.Fatalf("expected both name and id filters to return snapshots; name=%d id=%d", len(byName), len(byID))
	}
	if byName[0].ID != "named-run" {
		t.Fatalf("group snapshot ID should stay the name for back-compat, got %q", byName[0].ID)
	}
}
