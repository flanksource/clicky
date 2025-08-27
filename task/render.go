package task

import (
	"fmt"
	"os"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/muesli/termenv"
)

func (tm *Manager) Render() {
	output := termenv.NewOutput(os.Stderr)
	isInteractive := tm.isInteractive
	noProgress := tm.noProgress

	if len(tm.tasks) == 0 {
		return
	}

	// Only use ANSI escape codes if we're in interactive mode
	if !noProgress && isInteractive {
		tm.mu.Lock()
		defer tm.mu.Unlock()
		output.ClearScreen()
		// Render the current state
		rendered := tm.Pretty().ANSI()
		fmt.Fprint(os.Stderr, rendered)

	} else {

		for _, task := range tm.tasks {
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

	if len(tm.tasks) == 0 {
		return api.Text{Content: "No tasks running"}
	}

	text := api.Text{Content: ""}
	for _, task := range tm.tasks {
		text.Children = append(text.Children, task.Pretty().Append("\n", "").Indent(2))
	}

	return text
}
