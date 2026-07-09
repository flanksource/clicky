package task

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/flanksource/commons/logger"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/text"
)

// fakeLiveRenderer returns fixed content so tests can assert the manager draws
// the caller's block instead of the default task tree. It records the task
// count it was handed so tests can confirm the snapshot is passed through.
type fakeLiveRenderer struct {
	live      string
	final     string
	liveTasks int
}

func (f *fakeLiveRenderer) RenderLive(tasks []*Task) api.Text {
	f.liveTasks = len(tasks)
	return api.Text{Content: f.live}
}

func (f *fakeLiveRenderer) RenderFinal(tasks []*Task) api.Text {
	return api.Text{Content: f.final}
}

func addCompletedTask(tm *Manager, name string) {
	task := tm.newTask(name)
	task.SetStatus(StatusSuccess)
	task.completed.Store(true)
	task.dirty.Store(true)
	tm.mu.Lock()
	tm.tasks = append(tm.tasks, task)
	tm.mu.Unlock()
}

// When a LiveRenderer is installed, the live-tick content comes from it and the
// default task-tree formatting (the task names) is absent.
func TestLiveRenderer_ReplacesLiveContent(t *testing.T) {
	tm := newTestManager(1)
	t.Cleanup(func() { close(tm.shutdown) })

	addCompletedTask(tm, "internal-task-name")

	fake := &fakeLiveRenderer{live: "CUSTOM STATUS TABLE"}
	tm.setLiveRenderer(fake)

	rendered := text.StripANSI(tm.renderLiveText().String())

	if !strings.Contains(rendered, "CUSTOM STATUS TABLE") {
		t.Errorf("expected custom renderer content, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "internal-task-name") {
		t.Errorf("default task-tree content leaked through custom renderer:\n%s", rendered)
	}
	if fake.liveTasks != 1 {
		t.Errorf("renderer should receive the 1-task snapshot, got %d", fake.liveTasks)
	}
}

// A nil renderer leaves the default task-tree output untouched (regression
// guard: the hook must be opt-in).
func TestLiveRenderer_NilFallsBackToDefault(t *testing.T) {
	tm := newTestManager(1)
	t.Cleanup(func() { close(tm.shutdown) })

	addCompletedTask(tm, "default-task-name")

	rendered := text.StripANSI(tm.renderLiveText().String())
	if !strings.Contains(rendered, "default-task-name") {
		t.Errorf("nil renderer should produce default task-tree output, got:\n%s", rendered)
	}
}

// renderFinal must use RenderFinal, not RenderLive or the default tree.
func TestLiveRenderer_FinalUsesRenderFinal(t *testing.T) {
	originalStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	// Redirect stderr before creating the manager so its lipgloss renderer
	// (constructed from os.Stderr in newManager) writes into the pipe.
	os.Stderr = w
	capture := &bytes.Buffer{}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(capture, r)
		close(done)
	}()

	tm := newTestManager(1)
	t.Cleanup(func() { close(tm.shutdown) })

	addCompletedTask(tm, "task-x")
	tm.setLiveRenderer(&fakeLiveRenderer{live: "LIVE-ONLY", final: "FINAL-SUMMARY"})

	tm.renderFinal(false)

	os.Stderr = originalStderr
	_ = w.Close()
	<-done
	_ = r.Close()

	out := text.StripANSI(capture.String())
	if !strings.Contains(out, "FINAL-SUMMARY") {
		t.Errorf("renderFinal should emit RenderFinal content, got:\n%s", out)
	}
	if strings.Contains(out, "LIVE-ONLY") {
		t.Errorf("renderFinal must not emit RenderLive content, got:\n%s", out)
	}
}

// PlainRender (non-interactive) emits the whole custom block once per tick,
// not the per-task dirty loop.
func TestLiveRenderer_PlainRenderEmitsWholeBlock(t *testing.T) {
	originalStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	// Redirect stderr before creating the manager so its lipgloss renderer
	// writes into the pipe.
	os.Stderr = w
	capture := &bytes.Buffer{}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(capture, r)
		close(done)
	}()

	tm := newTestManager(1)
	t.Cleanup(func() { close(tm.shutdown) })

	// Three tasks; only one dirty. The default PlainRender would print just the
	// dirty one. The custom renderer renders one fixed block regardless.
	for i := 0; i < 3; i++ {
		task := tm.newTask(fmt.Sprintf("file-%d", i))
		task.SetStatus(StatusSuccess)
		task.completed.Store(true)
		task.dirty.Store(i == 1)
		tm.mu.Lock()
		tm.tasks = append(tm.tasks, task)
		tm.mu.Unlock()
	}
	tm.setLiveRenderer(&fakeLiveRenderer{live: "WHOLE BLOCK"})

	tm.PlainRender()

	os.Stderr = originalStderr
	_ = w.Close()
	<-done
	_ = r.Close()

	out := text.StripANSI(capture.String())
	if !strings.Contains(out, "WHOLE BLOCK") {
		t.Errorf("PlainRender should emit the custom block, got:\n%s", out)
	}
	if strings.Contains(out, "file-0") || strings.Contains(out, "file-1") {
		t.Errorf("custom renderer should replace per-task lines, got:\n%s", out)
	}
}

// SetLiveRenderer is process-global and round-trips: install then clear.
func TestSetLiveRenderer_RoundTrip(t *testing.T) {
	t.Cleanup(func() { SetLiveRenderer(nil) })

	fake := &fakeLiveRenderer{live: "x"}
	SetLiveRenderer(fake)
	if global.getLiveRenderer() != fake {
		t.Fatalf("SetLiveRenderer did not install the renderer")
	}
	SetLiveRenderer(nil)
	if global.getLiveRenderer() != nil {
		t.Fatalf("SetLiveRenderer(nil) did not clear the renderer")
	}
}

// With a custom renderer active, a logger line emitted between ticks is still
// accounted for by the serializer, so the next ClearLines widens to cover it.
// This is the property that fixes the original corruption: the hook must not
// bypass the line-accounting that keeps the redraw in sync.
func TestLiveRenderer_LoggerLinesStillAccounted(t *testing.T) {
	tm := &Manager{}
	tm.installLogSerializer()
	t.Cleanup(tm.uninstallLogSerializer)

	tm.setLiveRenderer(&fakeLiveRenderer{live: "block"})

	logger.Infof("between-ticks log line")

	// The serializer counted the newline the logger emitted; a tick reads it
	// via TakeLinesWritten to widen ClearLines. Without accounting this is 0
	// and the next frame would stack.
	if got := tm.logSerializer.TakeLinesWritten(); got < 1 {
		t.Errorf("expected logger line to be counted for ClearLines widening, got %d", got)
	}
}
