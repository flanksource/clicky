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

// runWrite is one pending persist. Snapshots is set when the caller already
// holds them and the group is about to be evicted; otherwise the writer resolves
// the id itself, off whatever lock the caller was under.
type runWrite struct {
	id        string
	snapshots []TaskSnapshot
}

// runQueue holds the pending writes for one installed store. Writes coalesce by
// run id rather than queueing behind each other, so a store slower than run
// completion falls behind by work and never by data: the newest snapshot of a
// run supersedes the pending older one instead of being dropped for it.
type runQueue struct {
	mu      sync.Mutex
	pending map[string]runWrite
	order   []string
	closed  bool

	// wake carries at most one notification: the writer drains everything it
	// finds each time it looks, so further wakeups would only spin it.
	wake chan struct{}
	done chan struct{}
}

func newRunQueue() *runQueue {
	return &runQueue{
		pending: map[string]runWrite{},
		wake:    make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
}

// add queues one write. It never blocks on the store: callers hold the manager
// lock or a group lock, neither of which may wait on IO.
func (q *runQueue) add(write runWrite) {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	existing, queued := q.pending[write.id]
	if !queued {
		q.order = append(q.order, write.id)
	} else if write.snapshots == nil {
		// An id-only write resolves the group when the writer reaches it. Keep
		// the snapshots an eviction already captured: after eviction there is
		// no group left to resolve, and losing them loses the run.
		write.snapshots = existing.snapshots
	}
	q.pending[write.id] = write
	q.mu.Unlock()

	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// take returns the write that has been pending longest.
func (q *runQueue) take() (runWrite, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.order) > 0 {
		id := q.order[0]
		q.order = q.order[1:]
		write, ok := q.pending[id]
		if !ok {
			continue
		}
		delete(q.pending, id)
		return write, true
	}
	return runWrite{}, false
}

// close stops the queue accepting work. It does not wait for the writer — the
// caller does that on done, once it has released whatever lock it holds.
func (q *runQueue) close() {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()

	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// drained reports whether the queue is closed and empty, which is the only
// condition under which the writer has nothing left to do.
func (q *runQueue) drained() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed && len(q.order) == 0
}

var (
	storeMu     sync.RWMutex
	activeStore Store
	activeQueue *runQueue
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
	previous := activeQueue
	if previous != nil {
		// Closed under the same lock enqueueRunWrite reads the queue under, so
		// a concurrent write lands either in a queue that is still draining or
		// in the new one — never in one nobody is left to serve.
		previous.close()
	}
	activeStore = store

	var queue *runQueue
	if store != nil {
		queue = newRunQueue()
	}
	activeQueue = queue
	storeMu.Unlock()

	// Outside the lock: the outgoing writer may still be in SaveRun, and
	// holding storeMu across that would stall every task retiring behind it.
	if previous != nil {
		<-previous.done
	}
	if store != nil {
		go runWriter(ctx, store, queue)
	}
}

// CurrentStore returns the installed store, or nil.
func CurrentStore() Store {
	storeMu.RLock()
	defer storeMu.RUnlock()
	return activeStore
}

// runWriter persists queued runs until the queue is closed or ctx is done.
// Either way it first drains what it has already accepted: a queued write has
// no later retry, and an evicted run's snapshots exist nowhere else once its
// group is gone, so honouring the cancellation ahead of them loses runs the
// queue never overflowed on.
func runWriter(ctx context.Context, store Store, q *runQueue) {
	defer close(q.done)

	writeCtx := ctx
	for {
		for {
			write, ok := q.take()
			if !ok {
				break
			}
			persistRun(writeCtx, store, write)
		}
		if q.drained() {
			return
		}
		select {
		case <-q.wake:
		case <-ctx.Done():
			// New work would have no writer left to serve it, and the writes
			// already accepted cannot go out under a cancelled context, so the
			// queue is closed and flushed under one carrying only its values.
			q.close()
			writeCtx = context.WithoutCancel(ctx)
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

// enqueueRunWrite hands a run to the background writer. It never blocks on the
// store: it is called from GC and from the terminal transition, both of which
// hold locks the store must not wait behind. The read lock is held across the
// hand-off so the queue cannot be retired out from under it.
func enqueueRunWrite(write runWrite) {
	storeMu.RLock()
	defer storeMu.RUnlock()
	if activeQueue == nil {
		return
	}
	activeQueue.add(write)
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
