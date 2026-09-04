package task

// OnBeforeGC, if non-nil, is called with each group's full snapshot just before
// GCRuns removes it from the in-memory manager. The callback receives the
// group's stable ID and the full snapshot slice (group + child tasks). It is
// called while global.mu is held, so it must not call back into the task
// package.
//
// This is an eviction hook, not a persistence mechanism. To store finished runs
// durably, install a Store with SetStore instead: Store.SaveRun is handed the
// same snapshot slice from a background goroutine with no lock held, so it may
// block on IO, and it fires the moment a run finishes rather than at eviction
// runRetention later. Reach for OnBeforeGC when the work has to happen at
// eviction specifically — releasing resources keyed by run id, dropping caches,
// cleaning up scratch directories.
//
// The two are independent: when both are installed, both are called.
var OnBeforeGC func(groupID string, snapshots []TaskSnapshot)

// notifyBeforeGC delivers an evicted group to the hook, if one is installed.
func notifyBeforeGC(groupID string, snapshots []TaskSnapshot) {
	if OnBeforeGC == nil {
		return
	}
	OnBeforeGC(groupID, snapshots)
}
