package task

import (
	"testing"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
)

// A group that takes more work after going terminal — a sync that adds each
// phase once the previous one finishes — is running again. Keeping the first
// finishedAt would let GC evict it mid-run once that stale time aged past the
// retention period, and would leave the store holding the transient one-phase
// "success" instead of the run's real outcome.
func TestReopenedGroupPersistsFinalCompletion(t *testing.T) {
	withTestGlobal(t)
	store := installTestStore(t)

	g := StartGroup[any]("phased-run", WithKind("test"), WithConcurrency(1))
	runGroupToCompletion(t, g, "phase-1")
	if first := store.await(t); first.Total != 1 {
		t.Fatalf("first terminal save Total = %d, want 1", first.Total)
	}

	release := make(chan struct{})
	g.Add("phase-2", func(ctx flanksourceContext.Context, tk *Task) (any, error) {
		<-release
		tk.Success()
		return nil, nil
	})
	// Errorf, not Fatal: phase-2 is blocked on release until it is closed below.
	if !g.FinishedAt().IsZero() {
		t.Error("adding work to a finished group must clear FinishedAt")
	}
	_ = SnapshotGroup(g.Group)
	if !g.FinishedAt().IsZero() {
		t.Error("a group with pending work must not be observed as finished")
	}

	close(release)
	deadline := time.After(storeWriteTimeout)
	for g.FinishedAt().IsZero() {
		select {
		case <-deadline:
			t.Fatalf("group %q did not finish again; status=%s", g.Name(), g.Status())
		case <-time.After(10 * time.Millisecond):
			_ = SnapshotGroup(g.Group)
		}
	}

	final := store.await(t)
	if final.Total != 2 || final.Completed != 2 || final.Status != string(StatusSuccess) {
		t.Fatalf("final save = total %d, completed %d, status %q; want 2, 2, %q",
			final.Total, final.Completed, final.Status, StatusSuccess)
	}
}
