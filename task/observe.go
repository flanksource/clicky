package task

import (
	"sync"
	"sync/atomic"

	"github.com/flanksource/commons/logger"
)

type TaskPhase string

const (
	TaskStatusChanged TaskPhase = "status"
	TaskProgressed    TaskPhase = "progress"
)

// TaskEvent is one task transition. Status is the task's status when the
// event was published; Snapshot builds the full view on demand because it
// copies logs and output, which most observers filter out before needing.
type TaskEvent struct {
	Phase  TaskPhase
	Status Status
	Task   *Task
}

func (e TaskEvent) Snapshot() TaskSnapshot { return SnapshotTask(e.Task, e.Task.parent) }

type taskObserver struct {
	deliver  func(TaskEvent)
	inflight sync.WaitGroup
}

var taskObservers = struct {
	mu        sync.RWMutex
	next      uint64
	count     atomic.Int64
	observers map[uint64]*taskObserver
}{observers: make(map[uint64]*taskObserver)}

// ObserveTasks registers a process-wide observer of task status transitions
// and progress. Delivery is synchronous on the publishing goroutine with no
// task lock held, and ordered per task, so deliver must not block. Release
// waits for in-flight deliveries and must not be called from a delivery.
func ObserveTasks(deliver func(TaskEvent)) (release func()) {
	if deliver == nil {
		panic("task observer must not be nil")
	}
	observer := &taskObserver{deliver: deliver}
	taskObservers.mu.Lock()
	taskObservers.next++
	id := taskObservers.next
	taskObservers.observers[id] = observer
	taskObservers.count.Add(1)
	taskObservers.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			taskObservers.mu.Lock()
			delete(taskObservers.observers, id)
			taskObservers.count.Add(-1)
			taskObservers.mu.Unlock()
			observer.inflight.Wait()
		})
	}
}

// publishStatus delivers a status event when the task's status differs from
// the last one published. It must be called without t.mu held.
func (t *Task) publishStatus() {
	if taskObservers.count.Load() == 0 {
		return
	}
	t.observeMu.Lock()
	defer t.observeMu.Unlock()
	status := t.Status()
	if status == t.observedStatus {
		return
	}
	t.observedStatus = status
	publishTask(TaskEvent{Phase: TaskStatusChanged, Status: status, Task: t})
}

func (t *Task) publishProgress() {
	if taskObservers.count.Load() == 0 {
		return
	}
	t.observeMu.Lock()
	defer t.observeMu.Unlock()
	publishTask(TaskEvent{Phase: TaskProgressed, Status: t.Status(), Task: t})
}

func publishTask(event TaskEvent) {
	taskObservers.mu.RLock()
	delivers := make([]*taskObserver, 0, len(taskObservers.observers))
	for _, observer := range taskObservers.observers {
		observer.inflight.Add(1)
		delivers = append(delivers, observer)
	}
	taskObservers.mu.RUnlock()
	for _, observer := range delivers {
		func() {
			defer observer.inflight.Done()
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Errorf("task observer panicked: %v", recovered)
				}
			}()
			observer.deliver(event)
		}()
	}
}
