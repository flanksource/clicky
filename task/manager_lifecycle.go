package task

import (
	"fmt"
	"os"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/commons/logger"
	"golang.org/x/term"
)

// renderLifecycleState tracks the render loop lifecycle so it can be
// restarted: a batch of tasks enqueued after Wait()/stopRender starts a
// fresh loop instead of rendering nothing. Guarded by Manager.mu.
type renderLifecycleState int

const (
	renderIdle renderLifecycleState = iota
	renderRunning
	renderStopping
)

// startRenderLoop starts the unified render loop for both interactive and
// non-interactive modes. Safe to call repeatedly and concurrently with
// stopRender: the renderState machine ensures at most one loop runs, and a
// stopped loop is restarted by a later enqueue.
func (tm *Manager) startRenderLoop() {
	if tm.noRender.Load() {
		return
	}
	acquiredTTY := false
	if tm.isInteractive.Load() {
		if !globalANSITerminal.tryAcquireTaskRenderer(tm) {
			return
		}
		acquiredTTY = true
	}

	tm.mu.Lock()
	if tm.renderState != renderIdle {
		tm.mu.Unlock()
		if acquiredTTY {
			globalANSITerminal.releaseTaskRenderer(tm)
		}
		return
	}
	tm.renderState = renderRunning
	tm.stopRenderCh = make(chan struct{})
	tm.renderDone = make(chan struct{})
	tm.renderOwnsTTY = acquiredTTY

	// Route flanksource/commons/logger output through a writer that
	// serializes against tick renders. Without this, a logger.Infof between
	// two ticks shifts the cursor without updating lastLines, so the next
	// tick's ClearLines undercounts and leaves stale frame lines behind.
	// Restored in stopRender.
	tm.installLogSerializer()
	tm.mu.Unlock()

	go tm.renderLoop()
}

// installLogSerializer swaps the commons logger writer for a mutex-guarded
// serializer that shares tm.bufferMutex with the renderer. Caller must pair
// with uninstallLogSerializer (done by stopRender).
func (tm *Manager) installLogSerializer() {
	prev := logger.GetOutput()
	tm.savedLogOutput = prev
	ser := newLogSerializingWriter(&tm.bufferMutex, prev, tm.isInteractive.Load())
	tm.logSerializer = ser
	logger.SetOutput(ser)
}

// uninstallLogSerializer restores the pre-renderer logger writer. No-op if
// installLogSerializer was never called (e.g. noRender).
func (tm *Manager) uninstallLogSerializer() {
	if tm.savedLogOutput == nil {
		return
	}
	// A log line can land after the loop's final tick; emit it below the
	// final frame rather than dropping it.
	tm.bufferMutex.Lock()
	tm.logSerializer.FlushPending()
	tm.bufferMutex.Unlock()
	logger.SetOutput(tm.savedLogOutput)
	tm.savedLogOutput = nil
	tm.logSerializer = nil
}

// renderLoop is the unified loop for both interactive and non-interactive modes.
// Interactive: hides cursor, uses ClearLines for in-place updates.
// Non-interactive: delegates to PlainRender().
func (tm *Manager) renderLoop() {
	defer close(tm.renderDone)
	defer func() {
		if r := recover(); r != nil {
			tm.cleanupTerminal()
			tm.releaseRenderTerminal()
			panic(r)
		}
	}()

	output := tm.renderer.Output()
	if tm.isInteractive.Load() {
		tm.bufferMutex.Lock()
		output.HideCursor()
		tm.bufferMutex.Unlock()
	}

	lastLines := -1
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-tm.stopRenderCh:
			if tm.isInteractive.Load() {
				// Final in-place render so the visible frame reflects
				// completed task states even if stop fired between ticks.
				// interactiveRender atomically ClearLines(lastLines) then
				// writes the fresh content, avoiding the double-emit that
				// would occur if we cleared here and also called
				// renderFinal below in stopRender. final=true lifts the
				// terminal-height cap so problem tasks are never clipped
				// in the frame that persists in scrollback.
				tm.interactiveRender(lastLines, true)
			} else {
				tm.PlainRender()
			}
			return
		case <-ticker.C:
			if tm.isInteractive.Load() {
				lastLines = tm.interactiveRender(lastLines, false)
			} else {
				tm.PlainRender()
			}
		}
	}
}

// stopRender signals the render loop to stop, waits for teardown, and re-arms
// the lifecycle so a later enqueue can start a fresh loop.
func (tm *Manager) stopRender() {
	tm.mu.Lock()
	switch tm.renderState {
	case renderRunning:
		tm.renderState = renderStopping
		ch := tm.stopRenderCh
		done := tm.renderDone
		stopped := make(chan struct{})
		tm.renderStopDone = stopped
		tm.mu.Unlock()

		close(ch)
		<-done
		tm.finishRenderTeardown(true)

		tm.mu.Lock()
		tm.renderState = renderIdle
		tm.renderStopDone = nil
		tm.mu.Unlock()
		close(stopped)

	case renderStopping:
		// A concurrent caller (e.g. Wait racing the shutdown hook) is mid-
		// teardown; wait for it to finish rather than returning while
		// terminal/logger cleanup is still in flight.
		stopped := tm.renderStopDone
		tm.mu.Unlock()
		<-stopped

	default: // renderIdle: loop never started (noProgress/CI) or already stopped.
		tm.mu.Unlock()
		tm.finishRenderTeardown(false)
	}
}

// finishRenderTeardown emits the final frame where needed and restores
// terminal and logger state. Shared by both stopRender paths: after a running
// loop exits (loopWasRunning), and when no loop ever ran — noProgress/CI mode
// prints its final tree here.
func (tm *Manager) finishRenderTeardown(loopWasRunning bool) {
	// In interactive mode the stop branch of renderLoop already emitted
	// the authoritative final frame (via interactiveRender, which
	// ClearLines(lastLines)+writes atomically). Calling renderFinal
	// here would append a second copy of the summary below the live
	// frame, doubling every summary line. PlainRender-based mode gets its
	// closing output from renderFinal: the loop's stop branch already
	// flushed every dirty task via PlainRender, so a running loop only
	// needs the one-line summary; when no loop ran, nothing has been
	// printed yet and the full tree is emitted.
	if !tm.noRender.Load() && !tm.isInteractive.Load() {
		tm.renderFinal(loopWasRunning)
	}
	tm.cleanupTerminal()
	// On the idle path, only tear down while still idle: a concurrent enqueue
	// may have restarted the loop, which still needs its freshly installed
	// serializer and TTY ownership (its own stopRender releases them).
	tm.mu.Lock()
	teardown := loopWasRunning || tm.renderState == renderIdle
	if teardown {
		tm.uninstallLogSerializer()
	}
	tm.mu.Unlock()
	if teardown {
		tm.releaseRenderTerminal()
		// The renderer no longer owns the TTY: emit stderr writes that
		// GatedStderr buffered during the render window (F4). They appear
		// below the final frame, still in arrival order and already
		// secret-redacted by the upstream log writer.
		flushGatedStderr()
	}
}

// cleanupTerminal restores terminal to a clean state
func (tm *Manager) cleanupTerminal() {
	if !tm.isInteractive.Load() || tm.noProgress.Load() || !tm.ownsRenderTerminal() {
		return
	}
	output := tm.renderer.Output()
	tm.bufferMutex.Lock()
	output.ShowCursor()
	output.Reset()
	tm.bufferMutex.Unlock()
	if tm.originalTermState != nil {
		if err := term.Restore(int(os.Stderr.Fd()), tm.originalTermState); err != nil {
			logger.Debugf("failed to restore terminal state: %v", err)
		}
	}
}

// renderFinal outputs the final task status once per lifecycle. When the plain
// render loop ran (loopRan), per-tick PlainRender output already covers every
// task, so only a one-line gray summary is emitted; when no loop ran
// (noProgress/CI), the full task tree is printed. A custom LiveRenderer keeps
// its full RenderFinal block on both paths. finalRendered makes a repeat
// stopRender with no newly enqueued tasks print nothing — enqueue resets it.
func (tm *Manager) renderFinal(loopRan bool) {
	if tm.noRender.Load() {
		return
	}
	tm.mu.Lock()
	if tm.finalRendered || len(tm.tasks) == 0 {
		tm.mu.Unlock()
		return
	}
	tm.finalRendered = true
	taskSnapshot := make([]*Task, len(tm.tasks))
	copy(taskSnapshot, tm.tasks)
	tm.mu.Unlock()

	var rendered api.Text
	if r := tm.getLiveRenderer(); r != nil {
		rendered = r.RenderFinal(taskSnapshot)
	} else if loopRan {
		rendered = plainSummaryText(taskSnapshot)
	} else {
		rendered = tm.prettyFromTasks(taskSnapshot)
	}
	if rendered.IsEmpty() {
		return
	}
	// Write through renderer.Output (original stderr captured at init) so
	// the final summary joins the live render stream and doesn't land in
	// the StartCapturingOutput pipe — otherwise buffered content flushed
	// at shutdown gets stacked below every already-rendered frame. Guard
	// with bufferMutex so a log-serializer write in flight doesn't split
	// the summary block.
	output := tm.renderer.Output()
	tm.bufferMutex.Lock()
	defer tm.bufferMutex.Unlock()
	if tm.noColor.Load() {
		fmt.Fprintln(output, rendered.String())
	} else {
		fmt.Fprintln(output, rendered.ANSI())
	}
}

func (tm *Manager) ownsRenderTerminal() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.renderOwnsTTY
}

func (tm *Manager) releaseRenderTerminal() {
	tm.mu.Lock()
	ownsTTY := tm.renderOwnsTTY
	tm.renderOwnsTTY = false
	tm.mu.Unlock()

	if ownsTTY {
		globalANSITerminal.releaseTaskRenderer(tm)
	}
}

// IsInteractiveRenderActive reports whether the global task manager's
// interactive render loop currently owns the terminal. Callers that
// write to os.Stderr can consult this to drop writes that would
// corrupt the live frame. The check is cheap (atomic load + RLock).
//
// Returns false when the manager is non-interactive (PlainRender mode),
// before the renderer has acquired the TTY, or after stopRender has
// released it.
func IsInteractiveRenderActive() bool {
	if global == nil {
		return false
	}
	if !global.isInteractive.Load() {
		return false
	}
	return global.ownsRenderTerminal()
}
