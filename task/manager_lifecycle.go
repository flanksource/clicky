package task

import (
	"fmt"
	"os"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/commons/logger"
	"golang.org/x/term"
)

// startRenderLoop starts the unified render loop for both interactive and non-interactive modes.
// Must not be called concurrently with stopRender.
func (tm *Manager) startRenderLoop() {
	if tm.noRender.Load() {
		return
	}
	if tm.isInteractive.Load() && !globalANSITerminal.tryAcquireTaskRenderer(tm) {
		return
	}

	tm.mu.Lock()
	tm.stopRenderCh = make(chan struct{})
	tm.renderDone = make(chan struct{})
	tm.renderOwnsTTY = tm.isInteractive.Load()
	tm.mu.Unlock()

	// Route flanksource/commons/logger output through a writer that
	// serializes against tick renders. Without this, a logger.Infof between
	// two ticks shifts the cursor without updating lastLines, so the next
	// tick's ClearLines undercounts and leaves stale frame lines behind.
	// Restored in stopRender.
	tm.installLogSerializer()

	go tm.renderLoop()
}

// installLogSerializer swaps the commons logger writer for a mutex-guarded
// serializer that shares tm.bufferMutex with the renderer. Caller must pair
// with uninstallLogSerializer (done by stopRender).
func (tm *Manager) installLogSerializer() {
	prev := logger.GetOutput()
	tm.savedLogOutput = prev
	ser := newLogSerializingWriter(&tm.bufferMutex, prev)
	tm.logSerializer = ser
	logger.SetOutput(ser)
}

// uninstallLogSerializer restores the pre-renderer logger writer. No-op if
// installLogSerializer was never called (e.g. noRender).
func (tm *Manager) uninstallLogSerializer() {
	if tm.savedLogOutput == nil {
		return
	}
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

	var lastLines int
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
				// renderFinal below in stopRender.
				tm.interactiveRender(lastLines)
			} else {
				tm.PlainRender()
			}
			return
		case <-ticker.C:
			if tm.isInteractive.Load() {
				lastLines = tm.interactiveRender(lastLines)
			} else {
				tm.PlainRender()
			}
		}
	}
}

// stopRender signals the render loop to stop and waits for cleanup
func (tm *Manager) stopRender() {
	tm.renderStopped.Do(func() {
		tm.mu.RLock()
		ch := tm.stopRenderCh
		done := tm.renderDone
		tm.mu.RUnlock()

		if ch != nil {
			close(ch)
			<-done
		}
		// In interactive mode the stop branch of renderLoop already emitted
		// the authoritative final frame (via interactiveRender, which
		// ClearLines(lastLines)+writes atomically). Calling renderFinal
		// here would append a second copy of the summary below the live
		// frame, doubling every summary line. PlainRender-based mode only
		// prints dirty tasks per tick, so renderFinal is still needed to
		// cover tasks that completed between the last tick and stop.
		if !tm.noRender.Load() && !tm.isInteractive.Load() {
			tm.renderFinal()
		}
		tm.cleanupTerminal()
		tm.uninstallLogSerializer()
		tm.releaseRenderTerminal()
	})
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

// renderFinal outputs the final task status
func (tm *Manager) renderFinal() {
	if tm.noRender.Load() {
		return
	}
	tm.mu.RLock()
	if len(tm.tasks) == 0 {
		tm.mu.RUnlock()
		return
	}
	taskSnapshot := make([]*Task, len(tm.tasks))
	copy(taskSnapshot, tm.tasks)
	tm.mu.RUnlock()

	var rendered api.Text
	if r := tm.getLiveRenderer(); r != nil {
		rendered = r.RenderFinal(taskSnapshot)
	} else {
		rendered = tm.prettyFromTasks(taskSnapshot)
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
