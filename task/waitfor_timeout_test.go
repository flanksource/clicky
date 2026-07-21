package task

import (
	"testing"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/stretchr/testify/assert"
)

// TestWaitForDoesNotInventAVerdictForARunningTask pins the contract that broke
// in production: WaitFor waits for the worker's verdict rather than imposing a
// deadline of its own. The old implementation gave up after a hardcoded 300s,
// marked the still-running task Failed, and then deadlocked building the error
// message — it re-received from the already-drained one-shot timeout channel
// while holding t.mu, so the task never completed and its worker never got the
// lock back. Callers whose tasks carry their own 5m timeout (every gavel linter)
// tied that deadline exactly.
func TestWaitForDoesNotInventAVerdictForARunningTask(t *testing.T) {
	const work = 250 * time.Millisecond
	const want = 7

	slow := StartTask[int]("slow", func(ctx flanksourceContext.Context, tk *Task) (int, error) {
		time.Sleep(work)
		return want, nil
	})

	start := time.Now()
	res := slow.WaitFor()
	waited := time.Since(start)

	assert.GreaterOrEqual(t, waited, work, "WaitFor returned before the task finished")
	assert.Equal(t, StatusSuccess, res.Status)
	assert.NoError(t, res.Error, "WaitFor must not attach a waiter-side timeout error")

	got, err := slow.GetResult()
	assert.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestWaitForReleasesTaskLock guards the symptom that wedged the hung gavel
// processes: WaitFor holding t.mu meant the worker blocked forever on
// t.mu.Lock(), so workersActive never dropped and task.Wait() span its 10ms
// poll for days. Any concurrent reader of the task must stay responsive.
func TestWaitForReleasesTaskLock(t *testing.T) {
	const work = 150 * time.Millisecond

	slow := StartTask[int]("locked", func(ctx flanksourceContext.Context, tk *Task) (int, error) {
		time.Sleep(work)
		return 1, nil
	})

	probed := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		for i := 0; i < 20; i++ {
			_ = slow.Status()
			_ = slow.Name()
		}
		probed <- time.Since(start)
	}()

	slow.WaitFor()

	select {
	case elapsed := <-probed:
		assert.Less(t, elapsed, work, "Status/Name blocked on a lock held by WaitFor")
	case <-time.After(work * 10):
		t.Fatal("concurrent Status/Name never returned; WaitFor is holding t.mu")
	}
}
