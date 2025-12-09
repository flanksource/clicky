package task

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/flanksource/clicky/api"
	"github.com/muesli/termenv"
)

// PlainRender outputs the current task statuses in plain text without any interactive / ANSI / console features
func (tm *Manager) PlainRender() {

	tm.mu.RLock()
	defer tm.mu.RUnlock()
	if len(tm.tasks) == 0 {
		return
	}

	// Create snapshot to avoid holding lock during rendering
	taskSnapshot := make([]*Task, len(tm.tasks))
	copy(taskSnapshot, tm.tasks)

	// noProgress mode: only print dirty tasks, never clear screen
	for _, task := range taskSnapshot {
		if task.PopDirty() {
			if tm.noColor.Load() {
				fmt.Fprintf(os.Stderr, "%s\n", task.Pretty().String())
			} else {
				fmt.Fprintf(os.Stderr, "%s\n", task.Pretty().ANSI())
			}
			if task.bufferedLogger != nil {
				task.bufferedLogger.ClearLogs()
			}
		}
	}

}

func (tm *Manager) Render() {
	if tm.noProgress.Load() {
		tm.PlainRender()
		return
	}

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

	rendered := tm.prettyFromTasks(taskSnapshot)
	var out string
	if tm.noColor.Load() {
		out = rendered.String()
	} else {
		out = rendered.ANSI()
	}

	// Enable alternate screen on first render to avoid scrollback pollution
	if !tm.altScreenActive {
		output.AltScreen()
		tm.altScreenActive = true
	}

	// Clear screen and reset cursor
	output.ClearScreen()
	output.MoveCursor(1, 1)

	out = lipgloss.NewStyle().MaxHeight(api.GetTerminalLines()).Render(out)

	fmt.Fprintf(outputWriter, "%s\n", out)

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
