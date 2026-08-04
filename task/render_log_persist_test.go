package task

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/flanksource/commons/logger"

	"github.com/flanksource/clicky/api"
)

// F5: logger lines emitted between interactive ticks must persist in
// scrollback above the live frame. The tick clears only the old frame,
// prints the pending log lines, then repaints the frame below them (the
// bubbletea tea.Println pattern) — the log lines are never cleared.
func TestInteractiveRender_LogLinesPersistAboveFrame(t *testing.T) {
	tm := newTestManager(1)
	t.Cleanup(func() { close(tm.shutdown) })
	tm.isInteractive.Store(true)

	// Route the renderer and the logger into the same recording buffer so
	// write ordering across both sinks is observable (in production both
	// are stderr).
	buf := &syncBuffer{}
	tm.renderer = lipgloss.NewRenderer(buf)

	origLog := logger.GetOutput()
	logger.SetOutput(buf)
	tm.installLogSerializer()
	t.Cleanup(func() {
		tm.uninstallLogSerializer()
		logger.SetOutput(origLog)
	})

	task := tm.newTask("frame-task")
	task.SetStatus(StatusSuccess)
	task.completed.Store(true)
	tm.mu.Lock()
	tm.tasks = append(tm.tasks, task)
	tm.mu.Unlock()

	frameRows := tm.interactiveRender(0, false)
	tick1 := buf.String()

	if _, err := tm.logSerializer.Write([]byte("LOG-BETWEEN-TICKS\n")); err != nil {
		t.Fatalf("serializer write: %v", err)
	}
	if got := buf.String(); got != tick1 {
		t.Fatalf("interactive log write must be buffered until the next tick; wrote %q", got[len(tick1):])
	}

	tm.interactiveRender(frameRows, false)
	tick2 := buf.String()[len(tick1):]

	logIdx := strings.Index(tick2, "LOG-BETWEEN-TICKS")
	frameIdx := strings.Index(tick2, "frame-task")
	if logIdx < 0 || frameIdx < 0 {
		t.Fatalf("tick must emit both the log line and the frame; got:\n%q", tick2)
	}
	if logIdx > frameIdx {
		t.Errorf("log line must be printed above the repainted frame; got:\n%q", tick2)
	}

	// The clear must cover only the old frame's rows; widening it would
	// erase the log line the moment the next tick fires.
	if ups := strings.Count(tick2, "\x1b[1A"); ups != frameRows {
		t.Errorf("tick cleared %d rows, want %d (previous frame only)", ups, frameRows)
	}

	tm.interactiveRender(frameRows, false)
	tick3 := buf.String()[len(tick1)+len(tick2):]
	if strings.Contains(tick3, "LOG-BETWEEN-TICKS") {
		t.Errorf("log line must be emitted exactly once, found again in tick 3:\n%q", tick3)
	}
}

// newTailTestManager builds an interactive manager whose renderer writes to
// a recording buffer, with fabricated capture state: entries "cap-line-0"
// .. "cap-line-7" in the buffer and 3 lines already dropped (11 total).
func newTailTestManager(t *testing.T, capturing bool) (*Manager, *syncBuffer) {
	t.Helper()
	tm := newTestManager(1)
	t.Cleanup(func() { close(tm.shutdown) })
	tm.isInteractive.Store(true)

	buf := &syncBuffer{}
	tm.renderer = lipgloss.NewRenderer(buf)

	tm.bufferMutex.Lock()
	tm.capturingOutput = capturing
	tm.outputDropped = 3
	for i := 0; i < 8; i++ {
		tm.outputBuffer = append(tm.outputBuffer, OutputEntry{Stream: "stdout", Line: "cap-line-" + string(rune('0'+i))})
	}
	tm.bufferMutex.Unlock()
	t.Cleanup(func() {
		tm.bufferMutex.Lock()
		tm.capturingOutput = false
		tm.bufferMutex.Unlock()
	})
	return tm, buf
}

func addTailTask(tm *Manager, name string, status Status) {
	task := tm.newTask(name)
	task.SetStatus(status)
	if status != StatusPending && status != StatusRunning {
		task.completed.Store(true)
	}
	tm.mu.Lock()
	tm.tasks = append(tm.tasks, task)
	tm.mu.Unlock()
}

// F6: while capture is active and work is running, the live frame shows a
// tail of the newest captured lines with an explicit "last N of M" header
// counting dropped lines too.
func TestInteractiveRender_ShowsCaptureTail(t *testing.T) {
	tm, buf := newTailTestManager(t, true)
	addTailTask(tm, "runner", StatusRunning)

	tm.interactiveRender(0, false)
	out := buf.String()

	if !strings.Contains(out, "cap-line-7") {
		t.Errorf("live frame must show the newest captured line, got:\n%q", out)
	}
	if strings.Contains(out, "cap-line-2") {
		t.Errorf("live tail must show only the last lines, got:\n%q", out)
	}
	if !strings.Contains(out, "last 5 of 11 lines") {
		t.Errorf("tail header must report tail size and total (incl. dropped), got:\n%q", out)
	}
}

// The tail disappears once nothing is running: the final frame must not
// duplicate lines the stop flush is about to replay in full.
func TestInteractiveRender_TailHiddenWhenNothingRunning(t *testing.T) {
	tm, buf := newTailTestManager(t, true)
	addTailTask(tm, "done-task", StatusSuccess)

	tm.interactiveRender(0, false)
	if out := buf.String(); strings.Contains(out, "cap-line") {
		t.Errorf("tail must be hidden when no task is running, got:\n%q", out)
	}
}

// No tail when capture isn't active, buffer residue notwithstanding.
func TestInteractiveRender_TailHiddenWhenNotCapturing(t *testing.T) {
	tm, buf := newTailTestManager(t, false)
	addTailTask(tm, "runner", StatusRunning)

	tm.interactiveRender(0, false)
	if out := buf.String(); strings.Contains(out, "cap-line") {
		t.Errorf("tail must be hidden when capture is inactive, got:\n%q", out)
	}
}

// F16: live ticks stay capped to the terminal height, but the final frame
// escapes the cap so problem tasks are never clipped at exit.
func TestFinalInteractiveRenderUncapped(t *testing.T) {
	api.SetTerminalLines(6)
	t.Cleanup(func() { api.SetTerminalLines(-1) })

	tm := newTestManager(1)
	t.Cleanup(func() { close(tm.shutdown) })
	tm.isInteractive.Store(true)
	buf := &syncBuffer{}
	tm.renderer = lipgloss.NewRenderer(buf)

	for i := 0; i < 10; i++ {
		addTailTask(tm, fmt.Sprintf("fail-%d", i), StatusFailed)
	}

	tm.interactiveRender(0, false)
	tick := buf.String()
	if strings.Contains(tick, "fail-9") {
		t.Fatalf("live tick must stay height-capped (fail-9 should be clipped), got:\n%q", tick)
	}

	tm.interactiveRender(0, true)
	final := buf.String()[len(tick):]
	for i := 0; i < 10; i++ {
		if !strings.Contains(final, fmt.Sprintf("fail-%d", i)) {
			t.Errorf("final frame must include fail-%d despite the terminal height cap", i)
		}
	}
}

// A log line that lands after the loop's final tick must be flushed by
// uninstallLogSerializer (below the final frame), not dropped.
func TestUninstallLogSerializer_FlushesPending(t *testing.T) {
	tm := &Manager{}
	tm.isInteractive.Store(true)

	buf := &syncBuffer{}
	origLog := logger.GetOutput()
	logger.SetOutput(buf)
	t.Cleanup(func() { logger.SetOutput(origLog) })
	tm.installLogSerializer()

	logger.Infof("late log line")
	if strings.Contains(buf.String(), "late log line") {
		t.Fatalf("interactive log write must be buffered until flushed, got %q", buf.String())
	}

	tm.uninstallLogSerializer()
	if !strings.Contains(buf.String(), "late log line") {
		t.Errorf("uninstall must flush pending log lines, got %q", buf.String())
	}
	if logger.GetOutput() != io.Writer(buf) {
		t.Errorf("uninstall must restore the previous logger output")
	}
}
