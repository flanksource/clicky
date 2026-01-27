package task

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flanksource/clicky/api"
)

type tickMsg time.Time
type shutdownMsg struct{}

type taskModel struct {
	manager  *Manager
	width    int
	height   int
	quitting bool
}

func newTaskModel(manager *Manager) taskModel {
	return taskModel{
		manager: manager,
		width:   manager.width,
		height:  api.GetTerminalLines(),
	}
}

func (m taskModel) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m taskModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		if m.quitting {
			return m, nil
		}
		return m, tickCmd()

	case shutdownMsg:
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

func (m taskModel) View() string {
	if m.quitting {
		return m.renderFinalView()
	}

	rendered := m.manager.Pretty()
	var out string
	if m.manager.noColor.Load() {
		out = rendered.String()
	} else {
		out = rendered.ANSI()
	}

	return lipgloss.NewStyle().MaxHeight(m.height).Render(out)
}

func (m taskModel) renderFinalView() string {
	return ""
	// rendered := m.manager.Pretty()
	// if m.manager.noColor.Load() {
	// 	return rendered.String()
	// }
	// return rendered.ANSI()
}
