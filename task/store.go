package task

import (
	"context"
	"sync"
	"time"

	"github.com/flanksource/commons/logger"
)

// Store is the durable half of the task manager. It extends RunSource — which
// already answers reads for runs this process no longer holds in memory — with
// the writes that put them there, and with the schedule definitions the
// scheduler replays after a restart.
//
// Every method is called from a background goroutine, never from a task's own
// goroutine and never while the manager holds a lock, so an implementation may
// block on IO. It must not call back into the task package.
type Store interface {
	RunSource

	// SaveRun persists one run: the group snapshot followed by its child task
	// snapshots. It is called when a run reaches a terminal status and again if
	// the run is still in memory when GC evicts it, so it must be idempotent —
	// the newest snapshot for a given id wins.
	SaveRun(ctx context.Context, groupID string, snapshots []TaskSnapshot) error

	ListSchedules(ctx context.Context) ([]Schedule, error)
	SaveSchedule(ctx context.Context, schedule Schedule) error
	DeleteSchedule(ctx context.Context, name string) error

	// RecordFire records one firing decision — a run that started, or one that
	// was deliberately not started. A skipped fire is a fact about the schedule
	// worth keeping, not an absence of one.
	RecordFire(ctx context.Context, name string, fire Fire) error
}

// FireOutcome is what the scheduler decided to do at a scheduled time.
type FireOutcome string

const (
	// FireStarted means the fire produced a run; RunID names it.
	FireStarted FireOutcome = "started"
	// FireSkipped means the previous run was still going and the schedule's
	// overlap policy is OverlapSkip.
	FireSkipped FireOutcome = "skipped"
	// FireCaughtUp means the fire replayed a scheduled time missed while the
	// process was down, under CatchUpOnce.
	FireCaughtUp FireOutcome = "caught-up"
	// FireFailed means the run could not be started at all.
	FireFailed FireOutcome = "failed"
)

// Fire is one entry in a schedule's firing history.
type Fire struct {
	// ScheduledFor is the cron instant this fire belongs to, which is not the
	// same as At when the fire is a catch-up or the scheduler ran late.
	ScheduledFor time.Time   `json:"scheduledFor"`
	At           time.Time   `json:"at"`
	Outcome      FireOutcome `json:"outcome"`
	RunID        string      `json:"runId,omitempty"`
	Reason       string      `json:"reason,omitempty"`
	Error        string      `json:"error,omitempty"`
}

// RunRecord is a run flattened for storage: the listing columns a store indexes
// and filters on, plus the full snapshot slice a drill-down needs. Keeping both
// means a listing never has to decode the snapshots to answer.
type RunRecord struct {
	RunMeta
	Snapshots []TaskSnapshot
}

// RunFromSnapshots builds a RunRecord from a group's snapshot slice — the group
// snapshot followed by its child tasks, as produced by SnapshotByID and by the
// GC hook. It reports false when the slice carries no group snapshot, which
// means there is nothing to persist.
func RunFromSnapshots(snapshots []TaskSnapshot) (RunRecord, bool) {
	for i := range snapshots {
		if snapshots[i].Type != "group" || snapshots[i].GroupID == "" {
			continue
		}
		return RunRecord{
			RunMeta:   RunMetaFromSnapshot(snapshots[i]),
			Snapshots: snapshots,
		}, true
	}
	return RunRecord{}, false
}

// runWriteQueueSize bounds the pending run writes. A full queue drops the write
// and says so: the alternative is blocking a task's completion on the store.
const runWriteQueueSize = 256

// runWrite is one pending persist. Snapshots is set when the caller already
// holds them and the group is about to be evicted; otherwise the writer resolves
// the id itself, off whatever lock the caller was under.
type runWrite struct {
	id        string
	snapshots []TaskSnapshot
}

var (
	storeMu     sync.RWMutex
	activeStore Store
	writes      chan runWrite
	writerDone  chan struct{}
)

// SetStore installs the durable store and starts the background writer that
// feeds it. The writer runs until ctx is done. Passing a nil store detaches the
// current one.
//
// Replacing or detaching a store closes its queue and waits for the writer to
// drain what is already in it, so a run that finished before the swap is not
// lost by it.
//
// Reads still go through the *WithSource handlers, which take a RunSource; the
// installed store satisfies that interface and can be passed to them directly.
func SetStore(ctx context.Context, store Store) {
	storeMu.Lock()
	previous := writerDone
	if writes != nil {
		close(writes)
		writes = nil
		writerDone = nil
	}
	activeStore = store

	var queue chan runWrite
	var done chan struct{}
	if store != nil {
		queue = make(chan runWrite, runWriteQueueSize)
		done = make(chan struct{})
		writes, writerDone = queue, done
	}
	storeMu.Unlock()

	// Outside the lock: the outgoing writer may still be in SaveRun, and
	// holding storeMu across that would stall every task retiring behind it.
	if previous != nil {
		<-previous
	}
	if store != nil {
		go runWriter(ctx, store, queue, done)
	}
}

// CurrentStore returns the installed store, or nil.
func CurrentStore() Store {
	storeMu.RLock()
	defer storeMu.RUnlock()
	return activeStore
}

func runWriter(ctx context.Context, store Store, queue <-chan runWrite, done chan<- struct{}) {
	defer close(done)
	for {
		select {
		case <-ctx.Done():
			return
		case write, ok := <-queue:
			if !ok {
				return
			}
			persistRun(ctx, store, write)
		}
	}
}

func persistRun(ctx context.Context, store Store, write runWrite) {
	snapshots := write.snapshots
	if snapshots == nil {
		// Resolved here rather than at enqueue time so the caller — which may
		// hold the manager lock — is never the one walking the group.
		snapshots = SnapshotByID(write.id)
	}
	record, ok := RunFromSnapshots(snapshots)
	if !ok {
		// The run was evicted before the writer reached it; GC already
		// persisted it on the way out.
		return
	}
	if err := store.SaveRun(ctx, record.ID, record.Snapshots); err != nil {
		logger.Warnf("task store: save run %s: %v", record.ID, err)
	}
}

// enqueueRunWrite hands a run to the background writer. It never blocks: it is
// called from GC and from the terminal transition, both of which hold locks the
// store must not wait behind.
func enqueueRunWrite(write runWrite) {
	storeMu.RLock()
	queue := writes
	storeMu.RUnlock()
	if queue == nil {
		return
	}
	select {
	case queue <- write:
	default:
		logger.Warnf("task store: write queue full, dropping run %s", write.id)
	}
}

// persistTerminalRun is called the first time a group is observed terminal, so
// a finished run is durable immediately rather than at GC ten minutes later.
func persistTerminalRun(groupID string) {
	if groupID == "" {
		return
	}
	enqueueRunWrite(runWrite{id: groupID})
}

// persistEvictedRun is called by GC with the snapshots already in hand, because
// after eviction there is no group left to resolve.
func persistEvictedRun(groupID string, snapshots []TaskSnapshot) {
	enqueueRunWrite(runWrite{id: groupID, snapshots: snapshots})
}
