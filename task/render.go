package task

import (
	"fmt"
	"os"
	"time"

	"github.com/flanksource/clicky/api"
)

func (tm *Manager) Render() {
	// output := termenv.NewOutput(os.Stderr)
	isInteractive := tm.isInteractive
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

	// Handle rendering based on interactive mode and progress settings
	if isInteractive && !noProgress {
		// Interactive mode with progress: clear screen and full redraw
		// output.ClearScreen()
		rendered := tm.prettyFromTasks(taskSnapshot).ANSI()
		fmt.Fprint(os.Stderr, rendered)
	} else if !noProgress {
		// Non-interactive but progress enabled: print full state
		rendered := tm.prettyFromTasks(taskSnapshot)
		if tm.noColor {
			fmt.Fprint(os.Stderr, rendered.String())
		} else {
			fmt.Fprint(os.Stderr, rendered.ANSI())
		}
	} else {
		// noProgress mode: only print dirty tasks, never clear screen
		for _, task := range taskSnapshot {
			if task.PopDirty() {
				if tm.noColor {
					fmt.Fprintf(os.Stderr, "%s\n", task.Pretty().String())
				} else {
					fmt.Fprintf(os.Stderr, "%s\n", task.Pretty().ANSI())
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

	text := api.Text{Content: ""}
	for _, task := range tasks {
		text.Children = append(text.Children, task.Pretty().Append("\n", "").Indent(2))
	}

	return text
}
