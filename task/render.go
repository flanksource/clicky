package task

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/flanksource/clicky/api"
)

// PlainRender outputs the current task statuses in plain text without any interactive / ANSI / console features
func (tm *Manager) PlainRender() {
	// A custom LiveRenderer owns the whole block; render it once per tick
	// rather than the per-task dirty loop so its layout (e.g. a status table)
	// stays coherent in non-interactive / piped output.
	if r := tm.getLiveRenderer(); r != nil {
		tm.plainRenderLive(r)
		return
	}

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

// plainRenderLive prints a custom LiveRenderer's block for one non-interactive
// tick, guarded by bufferMutex so a concurrent log-serializer write can't split
// the block.
func (tm *Manager) plainRenderLive(r LiveRenderer) {
	rendered := r.RenderLive(tm.snapshotTasks())
	output := tm.renderer.Output()
	tm.bufferMutex.Lock()
	defer tm.bufferMutex.Unlock()
	if tm.noColor.Load() {
		fmt.Fprintf(output, "%s\n", rendered.String())
	} else {
		fmt.Fprintf(output, "%s\n", rendered.ANSI())
	}
}

func (tm *Manager) Pretty() api.Text {
	if tm == nil {
		return api.Text{}
	}
	return tm.prettyFromTasks(tm.snapshotTasks())
}

// renderLiveText produces the content for one live tick: the installed
// LiveRenderer's output when set, otherwise the default task-tree formatting.
func (tm *Manager) renderLiveText() api.Text {
	if r := tm.getLiveRenderer(); r != nil {
		return r.RenderLive(tm.snapshotTasks())
	}
	return tm.Pretty()
}

// prettyFromTasks formats a snapshot of tasks without needing locks.
//
// Row selection follows a priority order so long task lists stay useful:
//
//  1. Problem tasks (failed / warning / cancelled) — always shown, never
//     truncated. Hiding a failure behind "... N more" is the opposite of
//     what the user needs at a glance.
//  2. Running tasks — always shown, never truncated. The user almost
//     always cares what's still in flight right now.
//  3. Success tasks — filled in with whatever vertical budget is left
//     after problem + running + pending. Collapsed with a "first + …
//     N more + last" summary when they don't all fit.
//  4. Pending tasks — up to 5 plus a summary line.
//
// The budget is the terminal height reported by api.GetTerminalLines().
// When problem + running alone exceed the budget we still render all of
// them (overflow is preferable to hiding signal the user can act on).
func (tm *Manager) prettyFromTasks(tasks []*Task) api.Text {
	if len(tasks) == 0 {
		return api.Text{}
	}

	var problemTasks, runningTasks, successTasks, pendingTasks []*Task
	for _, task := range tasks {
		switch task.Status() {
		case StatusPending:
			pendingTasks = append(pendingTasks, task)
		case StatusRunning:
			runningTasks = append(runningTasks, task)
		case StatusFailed, StatusWarning, StatusCancelled, StatusFAIL, StatusERR:
			problemTasks = append(problemTasks, task)
		default:
			successTasks = append(successTasks, task)
		}
	}

	text := api.Text{Content: ""}

	const maxPending = 5
	pendingLines := len(pendingTasks)
	if pendingLines > maxPending {
		pendingLines = maxPending + 1 // visible + summary line
	}

	termHeight := api.GetTerminalLines()
	// Room left for success tasks after we reserve lines for the always-
	// shown buckets. Keep at least 3 so the success block can still show
	// first + summary + last when it overflows.
	successBudget := termHeight - len(problemTasks) - len(runningTasks) - pendingLines
	successBudget = max(successBudget, 3)

	appendTask := func(t *Task) {
		text.Children = append(text.Children, t.Pretty().Append("\n", "").Indent(2))
	}
	appendSummary := func(content string) {
		text.Children = append(text.Children, api.Text{
			Content: content,
			Style:   "text-gray-400",
		}.Indent(2))
	}

	// 1. Problems first — users should see failures at the top of the pane.
	for _, t := range problemTasks {
		appendTask(t)
	}

	// 2. Running next — current in-flight work.
	for _, t := range runningTasks {
		appendTask(t)
	}

	// 3. Success tasks, collapsed if they don't fit.
	if len(successTasks) > successBudget {
		appendTask(successTasks[0])
		appendSummary(fmt.Sprintf("... and %d more\n", len(successTasks)-2))
		appendTask(successTasks[len(successTasks)-1])
	} else {
		for _, t := range successTasks {
			appendTask(t)
		}
	}

	// 4. Pending last — queued work that hasn't started.
	if len(pendingTasks) > maxPending {
		for i := range maxPending {
			appendTask(pendingTasks[i])
		}
		appendSummary(fmt.Sprintf("... %d more pending\n", len(pendingTasks)-maxPending))
	} else {
		for _, t := range pendingTasks {
			appendTask(t)
		}
	}

	return text
}

// interactiveRender renders tasks in-place using ANSI clear lines.
// Returns the number of lines rendered for the next cycle's ClearLines call.
func (tm *Manager) interactiveRender(lastLines int) int {
	rendered := tm.renderLiveText()
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
