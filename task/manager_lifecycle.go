package task

import (
	"fmt"
	"os"
	"time"

	"github.com/flanksource/commons/logger"
	"golang.org/x/term"
)

// startRenderLoop starts the unified render loop for both interactive and non-interactive modes.
// Must not be called concurrently with stopRender.
func (tm *Manager) startRenderLoop() {
	tm.mu.Lock()
	tm.stopRenderCh = make(chan struct{})
	tm.renderDone = make(chan struct{})
	tm.mu.Unlock()
	go tm.renderLoop()
}

// renderLoop is the unified loop for both interactive and non-interactive modes.
// Interactive: hides cursor, uses ClearLines for in-place updates.
// Non-interactive: delegates to PlainRender().
func (tm *Manager) renderLoop() {
	defer close(tm.renderDone)
	defer func() {
		if r := recover(); r != nil {
			tm.cleanupTerminal()
			panic(r)
		}
	}()

	output := tm.renderer.Output()
	if tm.isInteractive {
		output.HideCursor()
	}

	var lastLines int
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-tm.stopRenderCh:
			if tm.isInteractive {
				output.ClearLines(lastLines)
			} else {
				tm.PlainRender()
			}
			return
		case <-ticker.C:
			if tm.isInteractive {
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

		if ch == nil {
			return
		}
		close(ch)
		<-done
		tm.renderFinal()
		tm.cleanupTerminal()
	})
}

// cleanupTerminal restores terminal to a clean state
func (tm *Manager) cleanupTerminal() {
	if !tm.isInteractive {
		return
	}
	output := tm.renderer.Output()
	output.ShowCursor()
	output.Reset()
	if tm.originalTermState != nil {
		if err := term.Restore(int(os.Stderr.Fd()), tm.originalTermState); err != nil {
			logger.Debugf("failed to restore terminal state: %v", err)
		}
	}
}

// renderFinal outputs the final task status
func (tm *Manager) renderFinal() {
	tm.mu.RLock()
	if len(tm.tasks) == 0 {
		tm.mu.RUnlock()
		return
	}
	taskSnapshot := make([]*Task, len(tm.tasks))
	copy(taskSnapshot, tm.tasks)
	tm.mu.RUnlock()

	rendered := tm.prettyFromTasks(taskSnapshot)
	if tm.noColor.Load() {
		fmt.Fprintln(os.Stderr, rendered.String())
	} else {
		fmt.Fprintln(os.Stderr, rendered.ANSI())
	}
}
