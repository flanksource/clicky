package task

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"

	"github.com/flanksource/clicky/text"
)

func bufferedLogMessages(task *Task) []string {
	logs := task.getBufferedLogger().GetLogs()
	msgs := make([]string, 0, len(logs))
	for _, l := range logs {
		msgs = append(msgs, l.Message)
	}
	return msgs
}

// F13: lines logged through the task context's Logger land in the task's
// buffered logger — rendering under the task line in the tree — instead of
// interleaving with global logger output.
func TestContextLoggerRoutesToTaskBuffer(t *testing.T) {
	tm := newTestManager(1)
	t.Cleanup(func() { close(tm.shutdown) })

	task := tm.newTask("ctx-log-task")
	task.FlanksourceContext().Logger.Infof("via-context %d", 42)

	if msgs := bufferedLogMessages(task); !strings.Contains(strings.Join(msgs, "\n"), "via-context 42") {
		t.Fatalf("context log line must land in the task buffer, got %v", msgs)
	}
	if !task.PopDirty() {
		t.Errorf("a context log append at Info must mark the task dirty for streaming")
	}
}

// F13: the context handed to runFunc routes to the task buffer, including on
// the taskTimeout path where the worker rebuilds the flanksource context.
func TestRunFuncContextLoggerRoutesToTaskBuffer(t *testing.T) {
	cases := []struct {
		name string
		opts []Option
	}{
		{name: "plain context", opts: nil},
		{name: "task timeout context", opts: []Option{WithTaskTimeout(5 * time.Second)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tm := newTestManager(1)
			t.Cleanup(func() { close(tm.shutdown) })

			task := tm.newTask("runfunc-ctx-task", tc.opts...)
			task.runFunc = func(ctx flanksourceContext.Context, tk *Task) error {
				ctx.Logger.Infof("from-run-func")
				tk.Success()
				return nil
			}
			tm.enqueue(task)
			select {
			case <-task.doneChan:
			case <-time.After(5 * time.Second):
				t.Fatal("timeout waiting for task to complete")
			}

			if msgs := bufferedLogMessages(task); !strings.Contains(strings.Join(msgs, "\n"), "from-run-func") {
				t.Fatalf("runFunc ctx.Logger line must land in the task buffer, got %v", msgs)
			}
		})
	}
}

// F14: appends at streaming levels (Info and more severe) mark the task
// dirty so plain mode emits them on the next tick; Debug/Trace stay batched
// until the next status transition.
func TestLogAppendMarksDirtyAtStreamingLevels(t *testing.T) {
	tm := newTestManager(1)
	t.Cleanup(func() { close(tm.shutdown) })
	task := tm.newTask("dirty-log-task")
	task.PopDirty()

	task.Debugf("debug line")
	if task.PopDirty() {
		t.Errorf("Debugf must not mark the task dirty (stays batched)")
	}
	task.Tracef("trace line")
	if task.PopDirty() {
		t.Errorf("Tracef must not mark the task dirty (stays batched)")
	}
	task.Infof("info line")
	if !task.PopDirty() {
		t.Errorf("Infof must mark the task dirty for streaming")
	}
	task.Warnf("warn line")
	if !task.PopDirty() {
		t.Errorf("Warnf must mark the task dirty for streaming")
	}
	task.Errorf("error line")
	if !task.PopDirty() {
		t.Errorf("Errorf must mark the task dirty for streaming")
	}
}

// F14: a running task that logs without any status change streams the line
// on the next plain tick instead of batching until completion.
func TestPlainRenderStreamsLogAppends(t *testing.T) {
	originalStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w

	capture := &bytes.Buffer{}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(capture, r)
		close(done)
	}()

	tm := newTestManager(1)
	task := tm.newTask("stream-task")
	task.SetStatus(StatusRunning)
	tm.mu.Lock()
	tm.tasks = append(tm.tasks, task)
	tm.mu.Unlock()
	task.PopDirty()

	task.Infof("streamed-mid-run")
	tm.PlainRender()

	os.Stderr = originalStderr
	_ = w.Close()
	<-done
	_ = r.Close()

	if out := text.StripANSI(capture.String()); !strings.Contains(out, "streamed-mid-run") {
		t.Errorf("plain tick must stream the appended log line, got:\n%s", out)
	}
	close(tm.shutdown)
}
