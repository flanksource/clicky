package task

import (
	"fmt"
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

	// noProgress mode: only print dirty tasks, never clear screen. Route
	// writes through the renderer's Output (original stderr captured at
	// manager init) so live progress stays out of StartCapturingOutput's
	// buffer and appears in real time. Guard each line with bufferMutex so
	// a concurrent log-serializer write cannot split a single line write.
	output := tm.renderer.Output()
	for _, task := range taskSnapshot {
		if task.PopDirty() {
			tm.bufferMutex.Lock()
			if tm.noColor.Load() {
				fmt.Fprintf(output, "%s\n", task.Pretty().String())
			} else {
				fmt.Fprintf(output, "%s\n", task.Pretty().ANSI())
			}
			tm.bufferMutex.Unlock()
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

	// Strip trailing whitespace that lipgloss adds for line normalization.
	// This prevents bloated output in .cast recordings and piped output.
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	out = strings.Join(lines, "\n")

	output := tm.renderer.Output()
	// Hold bufferMutex for the full clear+write sequence so a concurrent
	// logger write routed through the installed log serializer cannot
	// land between the ClearLines and the new content. Both sides acquire
	// the same mutex; the render side holds it for the brief duration of
	// the clear + Fprint, well under a 250ms tick.
	tm.bufferMutex.Lock()
	// Widen the clear to cover any log lines the serializer emitted since
	// the last tick. Without this, a logger.Infof between ticks advances
	// the cursor past the tracked region; the next ClearLines(lastLines)
	// undercounts and leaves the top of the previous frame stacked above
	// the new one.
	extra := 0
	if tm.logSerializer != nil {
		extra = tm.logSerializer.TakeLinesWritten()
	}
	output.ClearLines(lastLines + extra)
	// Write content through the renderer's Output, which holds the original
	// stderr captured at manager init. Routing through bare os.Stderr here
	// would split writes across two sinks — the ClearLines bytes go direct
	// to the terminal while the content goes into StartCapturingOutput's
	// buffer and gets flushed at shutdown, stripping the clears from the
	// content and leaving every rendered frame stacked in the final output.
	fmt.Fprint(output, out)
	tm.bufferMutex.Unlock()

	return strings.Count(out, "\n")
}
