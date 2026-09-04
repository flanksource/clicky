package task

import (
	"sync"
	"testing"
	"time"
)

type gcHookCall struct {
	groupID   string
	snapshots []TaskSnapshot
}

// gcHookCapture records what OnBeforeGC received. The mutex guards against a
// concurrent GC tick rather than against the callback itself, which runs
// synchronously on whichever goroutine called GCRuns.
type gcHookCapture struct {
	mu    sync.Mutex
	calls []gcHookCall
}

// installGCHook points OnBeforeGC at a recorder for the duration of the test and
// clears it afterwards, since the hook is a package global shared by every test.
func installGCHook(t *testing.T) *gcHookCapture {
	t.Helper()
	capture := &gcHookCapture{}
	OnBeforeGC = func(groupID string, snapshots []TaskSnapshot) {
		capture.mu.Lock()
		defer capture.mu.Unlock()
		capture.calls = append(capture.calls, gcHookCall{groupID: groupID, snapshots: snapshots})
	}
	t.Cleanup(func() { OnBeforeGC = nil })
	return capture
}

func (c *gcHookCapture) recorded() []gcHookCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]gcHookCall(nil), c.calls...)
}

// expireRun backdates a finished group past the retention window so the next GC
// pass evicts it.
func expireRun(g *Group) {
	g.mu.Lock()
	g.finishedAt = time.Now().Add(-2 * runRetention)
	g.mu.Unlock()
}

func TestOnBeforeGCCalledWithSnapshotBeforeEviction(t *testing.T) {
	withTestGlobal(t)
	hook := installGCHook(t)

	g := StartGroup[any]("gc-hook-run", WithKind("test"), WithConcurrency(1))
	groupID := g.ID()
	runGroupToCompletion(t, g, "step-a", "step-b")
	expireRun(g.Group)

	GCRuns()

	calls := hook.recorded()
	if len(calls) != 1 {
		t.Fatalf("expected OnBeforeGC to fire once, fired %d times", len(calls))
	}
	if calls[0].groupID != groupID {
		t.Fatalf("OnBeforeGC groupID = %q, want %q", calls[0].groupID, groupID)
	}
	snaps := calls[0].snapshots
	if len(snaps) != 3 {
		t.Fatalf("expected 3 snapshots (1 group + 2 tasks), got %d", len(snaps))
	}
	if snaps[0].Type != "group" {
		t.Fatalf("first snapshot should be group, got %q", snaps[0].Type)
	}
	if snaps[0].Total != 2 {
		t.Fatalf("group snapshot Total = %d, want 2", snaps[0].Total)
	}
}

func TestOnBeforeGCNotCalledForLiveRuns(t *testing.T) {
	withTestGlobal(t)
	hook := installGCHook(t)

	g := StartGroup[any]("live-run", WithConcurrency(1))
	runGroupToCompletion(t, g, "step")

	GCRuns()

	if calls := hook.recorded(); len(calls) != 0 {
		t.Fatalf("OnBeforeGC should not fire for runs within the retention period, fired %d times", len(calls))
	}
}

// The hook is an eviction observer, not a persistence mechanism: installing one
// must not stop an installed Store from receiving the same evicted run.
func TestOnBeforeGCAndStoreBothReceiveEvictedRun(t *testing.T) {
	withTestGlobal(t)
	store := installTestStore(t)
	hook := installGCHook(t)

	g := StartGroup[any]("both-run", WithKind("test"), WithConcurrency(1))
	groupID := g.ID()
	runGroupToCompletion(t, g, "step")
	store.await(t) // the terminal save, which precedes the eviction this test is about
	expireRun(g.Group)

	GCRuns()

	calls := hook.recorded()
	if len(calls) != 1 {
		t.Fatalf("expected OnBeforeGC to fire once, fired %d times", len(calls))
	}
	if calls[0].groupID != groupID {
		t.Fatalf("OnBeforeGC groupID = %q, want %q", calls[0].groupID, groupID)
	}
	saved := store.await(t)
	if saved.ID != groupID {
		t.Fatalf("eviction save id = %q, want %q", saved.ID, groupID)
	}
	if len(saved.Snapshots) != len(calls[0].snapshots) {
		t.Fatalf("store and hook saw different snapshot counts: %d vs %d",
			len(saved.Snapshots), len(calls[0].snapshots))
	}
}

// A Store is told about a run twice — once when it finishes and again when it is
// evicted — but OnBeforeGC fires only on eviction. Callers that release
// resources from the hook depend on that: firing at completion would tear down a
// run still listed in memory for another ten minutes.
func TestOnBeforeGCNotCalledOnTerminalTransition(t *testing.T) {
	withTestGlobal(t)
	store := installTestStore(t)
	hook := installGCHook(t)

	g := StartGroup[any]("terminal-run", WithKind("test"), WithConcurrency(1))
	groupID := g.ID()
	runGroupToCompletion(t, g, "only-step")

	if saved := store.await(t); saved.ID != groupID {
		t.Fatalf("terminal save id = %q, want %q", saved.ID, groupID)
	}
	if calls := hook.recorded(); len(calls) != 0 {
		t.Fatalf("OnBeforeGC should not fire on the terminal transition, fired %d times", len(calls))
	}
}
