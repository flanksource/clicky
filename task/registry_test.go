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
