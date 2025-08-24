package task

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/flanksource/clicky/api"
)

// render is the main rendering loop for interactive display
func (tm *Manager) render() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-tm.stopRender:
			return
		case <-ticker.C:
			isInteractive := tm.isInteractive
			noProgress := tm.noProgress

			if len(tm.tasks) == 0 {
				continue
			}

			// Skip rendering if progress is disabled
			if noProgress {
				continue
			}

			// Only use ANSI escape codes if we're in interactive mode and colors are enabled
			if isInteractive {
				// Move cursor to home position and clear from cursor down
				// This is less aggressive than clearing the entire screen
				fmt.Fprint(os.Stderr, "\033[H\033[J")
				fmt.Fprint(os.Stderr, tm.Pretty().ANSI())
			} else if !isInteractive || tm.noColor {

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

// renderProgressBar renders a progress bar for a running task
func (tm *Manager) renderProgressBar(name string, value, maxValue int, duration string) string {
	barWidth := 30

	// If maxValue is 0 or unknown, show infinite spinner
	if maxValue == 0 {
		// Create a simple spinner animation
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		spinnerIndex := (int(time.Now().UnixNano()/1e8) % len(spinner))
		spinnerChar := spinner[spinnerIndex]

		// Show spinner with dots animation
		dots := strings.Repeat("•", (int(time.Now().UnixNano()/1e9)%4)+1) + strings.Repeat(" ", 3-(int(time.Now().UnixNano()/1e9)%4))
		bar := spinnerChar + " " + dots + strings.Repeat("░", barWidth-6)

		return tm.styles.running.Render(fmt.Sprintf("⟳ %s ", name)) +
			tm.styles.bar.Render(bar) +
			tm.styles.running.Render(fmt.Sprintf(" (%s)", duration))
	}

	// Regular progress bar
	percentage := float64(value) / float64(maxValue)
	if percentage > 1 {
		percentage = 1
	}

	filled := int(percentage * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	return tm.styles.running.Render(fmt.Sprintf("⟳ %s ", name)) +
		tm.styles.bar.Render(bar) +
		tm.styles.running.Render(fmt.Sprintf(" %3d%% (%s)", int(percentage*100), duration))
}
