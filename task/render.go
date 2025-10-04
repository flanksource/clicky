package task

import (
	"fmt"
	"os"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/muesli/termenv"
)

func (tm *Manager) Render() {
	// Lock rendering to prevent concurrent renders
	tm.renderMutex.Lock()
	defer tm.renderMutex.Unlock()

	// Determine the output writer - use original stderr if capturing, otherwise os.Stderr
	var outputWriter *os.File
	tm.bufferMutex.Lock()
	if tm.capturingOutput && tm.originalStderr != nil {
		outputWriter = tm.originalStderr
	} else {
		outputWriter = os.Stderr
	}
	tm.bufferMutex.Unlock()

	output := termenv.NewOutput(outputWriter)
	noProgress := tm.noProgress

	// Create a snapshot of tasks to avoid holding lock during I/O
	tm.mu.RLock()
	if len(tm.tasks) == 0 {
		tm.mu.RUnlock()
		return
	}

	// Create snapshot to avoid holding lock during rendering
	taskSnapshot := make([]*Task, len(tm.tasks))
	copy(taskSnapshot, tm.tasks)
	tm.mu.RUnlock()

	// Handle rendering based on progress settings
	if !noProgress {
		// Enable alternate screen on first render to avoid scrollback pollution
		if !tm.altScreenActive {
			output.AltScreen()
			tm.altScreenActive = true
		}

		// Clear screen and reset cursor
		output.ClearScreen()
		output.MoveCursor(1, 1)

		// Render begin marker
		fmt.Fprintln(outputWriter, "--- BEGIN RENDER ---")

		rendered := tm.prettyFromTasks(taskSnapshot)
		if tm.noColor {
			fmt.Fprint(outputWriter, rendered.String())
		} else {
			fmt.Fprint(outputWriter, rendered.ANSI())
		}

		// Render end marker
		fmt.Fprintln(outputWriter, "--- END RENDER ---")
	} else {
		// noProgress mode: only print dirty tasks, never clear screen
		for _, task := range taskSnapshot {
			if task.PopDirty() {
				if tm.noColor {
					fmt.Fprintf(outputWriter, "%s\n", task.Pretty().String())
				} else {
					fmt.Fprintf(outputWriter, "%s\n", task.Pretty().ANSI())
				}
			}
		}
	}
}

// render is the main rendering loop for interactive display
func (tm *Manager) render() {
	defer tm.renderDone.Store(true)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-tm.stopRender:
			tm.Render()
			return
		case <-ticker.C:
			tm.Render()
		}
	}
}

func (tm *Manager) Pretty() api.Text {
	if tm == nil {
		return api.Text{}
	}

	tm.mu.RLock()
	if len(tm.tasks) == 0 {
		tm.mu.RUnlock()
		return api.Text{Content: "No tasks running"}
	}

	// Create snapshot to avoid holding lock during formatting
	taskSnapshot := make([]*Task, len(tm.tasks))
	copy(taskSnapshot, tm.tasks)
	tm.mu.RUnlock()

	return tm.prettyFromTasks(taskSnapshot)
}

// prettyFromTasks formats a snapshot of tasks without needing locks
func (tm *Manager) prettyFromTasks(tasks []*Task) api.Text {
	if len(tasks) == 0 {
		return api.Text{Content: "No tasks running"}
	}

	// Separate pending and non-pending tasks
	var pendingTasks, nonPendingTasks []*Task
	for _, task := range tasks {
		if task.Status() == StatusPending {
			pendingTasks = append(pendingTasks, task)
		} else {
			nonPendingTasks = append(nonPendingTasks, task)
		}
	}

	text := api.Text{Content: ""}

	// Show all non-pending tasks
	for _, task := range nonPendingTasks {
		text.Children = append(text.Children, task.Pretty().Append("\n", "").Indent(2))
	}

	// Show only first 5 pending tasks if there are more than 5
	maxPending := 5
	if len(pendingTasks) > maxPending {
		for i := 0; i < maxPending; i++ {
			text.Children = append(text.Children, pendingTasks[i].Pretty().Append("\n", "").Indent(2))
		}
		remaining := len(pendingTasks) - maxPending
		text.Children = append(text.Children, api.Text{
			Content: fmt.Sprintf("... %d more pending\n", remaining),
			Style:   "text-gray-400",
		}.Indent(2))
	} else {
		for _, task := range pendingTasks {
			text.Children = append(text.Children, task.Pretty().Append("\n", "").Indent(2))
		}
	}

	return text
}
