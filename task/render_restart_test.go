package task

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/flanksource/commons/logger"

	"github.com/flanksource/clicky/text"
)

// syncBuffer is a mutex-guarded buffer safe for a concurrent io.Copy writer
// and test-side readers.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// newRestartManager builds a manager in plain (non-interactive) render mode
// whose renderer writes into a captured pipe: noProgress stays false so the
// render loop starts on enqueue, noColor keeps assertions stable.
func newRestartManager(t *testing.T) (*Manager, *syncBuffer) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	// The lipgloss renderer binds os.Stderr at construction time; swap it in
	// only for the constructor so live output lands in the pipe.
	origStderr := os.Stderr
	os.Stderr = w
	tm := newManagerWithConcurrency(2)
	os.Stderr = origStderr

	tm.noColor.Store(true)
	tm.isInteractive.Store(false)

	capture := &syncBuffer{}
	copyDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(capture, r)
		close(copyDone)
	}()

	origLogOutput := logger.GetOutput()
	t.Cleanup(func() {
		tm.stopRender()
		close(tm.shutdown)
		_ = w.Close()
		<-copyDone
		_ = r.Close()
		logger.SetOutput(origLogOutput)
	})

	return tm, capture
}

func runRestartTask(t *testing.T, tm *Manager, name string) {
	t.Helper()
	task := tm.newTask(name)
	task.runFunc = func(ctx flanksourceContext.Context, tk *Task) error {
		tk.Success()
		return nil
	}
	tm.enqueue(task)
	select {
	case <-task.doneChan:
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for task %q to complete", name)
	}
}

func waitForRenderOutput(t *testing.T, capture *syncBuffer, want string) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		if strings.Contains(text.StripANSI(capture.String()), want) {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %q in render output; got:\n%s",
				want, text.StripANSI(capture.String()))
		case <-tick.C:
		}
	}
}

// F10: the render lifecycle must be re-armable. After stopRender ends the
// first batch (as Wait/WaitSilent do), a later enqueue must start a fresh
// render loop: recreated stop channel, reinstalled commons-logger serializer,
// and visible tick output for the new batch.
func TestRenderLoopRestartsAfterStopRender(t *testing.T) {
	tm, capture := newRestartManager(t)

	runRestartTask(t, tm, "first-batch-task")
	tm.stopRender()
	waitForRenderOutput(t, capture, "first-batch-task")

	tm.mu.RLock()
	chBefore := tm.stopRenderCh
	tm.mu.RUnlock()

	runRestartTask(t, tm, "second-batch-task")

	tm.mu.RLock()
	chAfter := tm.stopRenderCh
	serializer := tm.logSerializer
	tm.mu.RUnlock()

	if chAfter == nil || chAfter == chBefore {
		t.Errorf("enqueue after stopRender must restart the render loop with a fresh stop channel")
	}
	if serializer == nil {
		t.Errorf("expected the commons logger serializer to be reinstalled for the restarted loop")
	} else if logger.GetOutput() != io.Writer(serializer) {
		t.Errorf("expected logger.GetOutput() to be the reinstalled serializer while the loop runs")
	}

	waitForRenderOutput(t, capture, "second-batch-task")
}

// F10(2): when noProgress is set at first enqueue (e.g. before CLI flags are
// applied), the lifecycle must not be consumed — after noProgress flips off,
// the next enqueue must start the render loop.
func TestRenderLoopStartsAfterNoProgressFlip(t *testing.T) {
	tm, capture := newRestartManager(t)
	tm.noProgress.Store(true)

	runRestartTask(t, tm, "before-flip-task")

	tm.mu.RLock()
	chBefore := tm.stopRenderCh
	tm.mu.RUnlock()
	if chBefore != nil {
		t.Fatalf("render loop must not start while noProgress is set")
	}

	tm.noProgress.Store(false)
	runRestartTask(t, tm, "after-flip-task")

	tm.mu.RLock()
	chAfter := tm.stopRenderCh
	tm.mu.RUnlock()
	if chAfter == nil {
		t.Errorf("enqueue after the noProgress flip must start the render loop")
	}

	waitForRenderOutput(t, capture, "after-flip-task")
}
