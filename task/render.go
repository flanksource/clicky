package task

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/flanksource/clicky/api"
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

func (tm *Manager) Pretty() api.Text {
	if tm == nil {
		return api.Text{}
	}

	tm.mu.RLock()
	if len(tm.tasks) == 0 {
		tm.mu.RUnlock()
		return api.Text{}
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
		return api.Text{}
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

	// Calculate how many pending lines will be shown
	maxPending := 5
	pendingLines := len(pendingTasks)
	if pendingLines > maxPending {
		pendingLines = maxPending + 1 // visible tasks + summary line
	}

	// Calculate available lines for completed tasks
	// Reserve space for pending tasks, use remaining terminal height for completed
	termHeight := api.GetTerminalLines()
	maxCompleted := termHeight - pendingLines
	maxCompleted = max(maxCompleted, 3) // always show at least first, summary, and last

	// Show completed tasks, collapsing if they exceed available space
	if len(nonPendingTasks) > maxCompleted {
		// Show first task
		text.Children = append(text.Children, nonPendingTasks[0].Pretty().Append("\n", "").Indent(2))
		// Show collapsed summary
		remaining := len(nonPendingTasks) - 2
		text.Children = append(text.Children, api.Text{
			Content: fmt.Sprintf("... and %d more\n", remaining),
			Style:   "text-gray-400",
		}.Indent(2))
		// Show last task
		text.Children = append(text.Children, nonPendingTasks[len(nonPendingTasks)-1].Pretty().Append("\n", "").Indent(2))
	} else {
		for _, task := range nonPendingTasks {
			text.Children = append(text.Children, task.Pretty().Append("\n", "").Indent(2))
		}
	}

	// Show only first maxPending pending tasks if there are more
	if len(pendingTasks) > maxPending {
		for i := range maxPending {
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

// interactiveRender renders tasks in-place using ANSI clear lines.
// Returns the number of lines rendered for the next cycle's ClearLines call.
func (tm *Manager) interactiveRender(lastLines int) int {
	rendered := tm.Pretty()
	var out string
	if tm.noColor.Load() {
		out = rendered.String()
	} else {
		out = rendered.ANSI()
	}

	out = lipgloss.NewStyle().MaxHeight(api.GetTerminalLines()).Render(out)

	output := tm.renderer.Output()
	output.ClearLines(lastLines)
	fmt.Fprint(os.Stderr, out)

	return strings.Count(out, "\n")
}
