// Package gumchoose vendors the choose implementation from charmbracelet/gum.
// It is adapted for local use in clicky's prompt APIs.
package gumchoose

import (
	"io"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type keymap struct {
	Down, Up, Right, Left, Home, End, ToggleAll, Toggle, Abort, Quit, Submit key.Binding
}

func defaultKeymap() keymap {
	return keymap{
		Down: key.NewBinding(
			key.WithKeys("down", "j", "ctrl+j", "ctrl+n"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k", "ctrl+k", "ctrl+p"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l", "ctrl+f"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h", "ctrl+b"),
		),
		Home: key.NewBinding(
			key.WithKeys("g", "home"),
		),
		End: key.NewBinding(
			key.WithKeys("G", "end"),
		),
		ToggleAll: key.NewBinding(
			key.WithKeys("a", "A", "ctrl+a"),
			key.WithHelp("ctrl+a", "select all"),
			key.WithDisabled(),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" ", "tab", "x", "ctrl+@"),
			key.WithHelp("x", "toggle"),
			key.WithDisabled(),
		),
		Abort: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "abort"),
		),
		Quit: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "quit"),
		),
		Submit: key.NewBinding(
			key.WithKeys("enter", "ctrl+q"),
			key.WithHelp("enter", "submit"),
		),
	}
}

func (k keymap) FullHelp() [][]key.Binding {
	return nil
}

func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Toggle,
		key.NewBinding(
			key.WithKeys("up", "down", "right", "left"),
			key.WithHelp("←↓↑→", "navigate"),
		),
		k.Submit,
		k.ToggleAll,
	}
}

// Options configures the vendored chooser.
type Options struct {
	Header           string
	Height           int
	Cursor           string
	ShowHelp         bool
	Limit            int
	Ordered          bool
	InitialIndex     int
	SelectedPrefix   string
	UnselectedPrefix string
}

// Result is the outcome of a chooser run.
type Result struct {
	Submitted bool
	Indexes   []int
}

type item struct {
	text     string
	selected bool
	order    int
}

type model struct {
	height int
	cursor string

	selectedPrefix   string
	unselectedPrefix string
	header           string

	items     []item
	quitting  bool
	submitted bool
	index     int
	limit     int
	ordered   bool

	numSelected  int
	currentOrder int

	paginator paginator.Model
	showHelp  bool
	help      help.Model
	keymap    keymap

	cursorStyle       lipgloss.Style
	headerStyle       lipgloss.Style
	itemStyle         lipgloss.Style
	selectedItemStyle lipgloss.Style
	paginatorStyle    lipgloss.Style
}

// Run executes the vendored chooser.
func Run(input io.Reader, output io.Writer, items []string, opts Options) (Result, error) {
	m := newModel(items, opts)

	program := tea.NewProgram(
		m,
		tea.WithInput(input),
		tea.WithOutput(output),
	)

	finalModel, err := program.Run()
	if err != nil {
		return Result{}, err
	}

	finished := finalModel.(model)
	return Result{
		Submitted: finished.submitted,
		Indexes:   finished.selectedIndexes(),
	}, nil
}

func newModel(options []string, opts Options) model {
	height := opts.Height
	if height <= 0 {
		height = 10
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 1
	}

	km := defaultKeymap()
	if limit > 1 {
		km.Toggle.SetEnabled(true)
		km.ToggleAll.SetEnabled(true)
	}

	p := paginator.New()
	p.Type = paginator.Dots
	p.PerPage = height
	p.SetTotalPages(len(options))

	items := make([]item, 0, len(options))
	for _, option := range options {
		items = append(items, item{text: option})
	}

	index := clamp(opts.InitialIndex, 0, max(len(items)-1, 0))
	p.Page = index / height

	theme := huh.ThemeCharm()
	selectedPrefix := defaultString(opts.SelectedPrefix, "◉ ")
	unselectedPrefix := defaultString(opts.UnselectedPrefix, "○ ")
	if limit <= 1 {
		selectedPrefix = ""
		unselectedPrefix = ""
	}

	return model{
		height:            height,
		cursor:            defaultString(opts.Cursor, "> "),
		selectedPrefix:    selectedPrefix,
		unselectedPrefix:  unselectedPrefix,
		header:            opts.Header,
		items:             items,
		index:             index,
		limit:             limit,
		ordered:           opts.Ordered,
		paginator:         p,
		showHelp:          opts.ShowHelp,
		help:              themedHelp(theme),
		keymap:            km,
		cursorStyle:       theme.Focused.SelectSelector.UnsetString(),
		headerStyle:       theme.Group.Title,
		itemStyle:         theme.Focused.UnselectedOption,
		selectedItemStyle: theme.Focused.SelectedOption,
		paginatorStyle:    theme.Group.Description,
	}
}

func themedHelp(theme *huh.Theme) help.Model {
	model := help.New()
	model.Styles = theme.Help
	return model
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, nil
	case tea.KeyMsg:
		start, end := m.paginator.GetSliceBounds(len(m.items))
		km := m.keymap

		switch {
		case key.Matches(msg, km.Down):
			m.index++
			if m.index >= len(m.items) {
				m.index = 0
				m.paginator.Page = 0
			}
			if m.index >= end {
				m.paginator.NextPage()
			}
		case key.Matches(msg, km.Up):
			m.index--
			if m.index < 0 {
				m.index = len(m.items) - 1
				m.paginator.Page = m.paginator.TotalPages - 1
			}
			if m.index < start {
				m.paginator.PrevPage()
			}
		case key.Matches(msg, km.Right):
			m.index = clamp(m.index+m.height, 0, len(m.items)-1)
			m.paginator.NextPage()
		case key.Matches(msg, km.Left):
			m.index = clamp(m.index-m.height, 0, len(m.items)-1)
			m.paginator.PrevPage()
		case key.Matches(msg, km.End):
			m.index = len(m.items) - 1
			m.paginator.Page = m.paginator.TotalPages - 1
		case key.Matches(msg, km.Home):
			m.index = 0
			m.paginator.Page = 0
		case key.Matches(msg, km.ToggleAll):
			if m.limit <= 1 {
				break
			}
			if m.numSelected < len(m.items) && m.numSelected < m.limit {
				m = m.selectAll()
			} else {
				m = m.deselectAll()
			}
		case key.Matches(msg, km.Quit):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, km.Abort):
			m.quitting = true
			return m, tea.Interrupt
		case key.Matches(msg, km.Toggle):
			if m.limit == 1 {
				break
			}
			if m.items[m.index].selected {
				m.items[m.index].selected = false
				m.numSelected--
			} else if m.numSelected < m.limit {
				m.items[m.index].selected = true
				m.items[m.index].order = m.currentOrder
				m.numSelected++
				m.currentOrder++
			}
		case key.Matches(msg, km.Submit):
			m.quitting = true
			if m.limit <= 1 && m.numSelected < 1 {
				m.items[m.index].selected = true
			}
			m.submitted = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.paginator, cmd = m.paginator.Update(msg)
	return m, cmd
}

func (m model) selectAll() model {
	for i := range m.items {
		if m.numSelected >= m.limit {
			break
		}
		if m.items[i].selected {
			continue
		}
		m.items[i].selected = true
		m.items[i].order = m.currentOrder
		m.numSelected++
		m.currentOrder++
	}
	return m
}

func (m model) deselectAll() model {
	for i := range m.items {
		m.items[i].selected = false
		m.items[i].order = 0
	}
	m.numSelected = 0
	m.currentOrder = 0
	return m
}

func (m model) selectedIndexes() []int {
	indexes := make([]int, 0, len(m.items))
	for index, item := range m.items {
		if item.selected {
			indexes = append(indexes, index)
		}
	}

	if m.ordered {
		slices.SortStableFunc(indexes, func(a, b int) int {
			return m.items[a].order - m.items[b].order
		})
	}

	return indexes
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder
	start, end := m.paginator.GetSliceBounds(len(m.items))

	for i, item := range m.items[start:end] {
		isCurrent := i == m.index%m.height
		if isCurrent {
			s.WriteString(m.cursorStyle.Render(m.cursor))
		} else {
			s.WriteString(strings.Repeat(" ", lipgloss.Width(m.cursor)))
		}

		switch {
		case item.selected:
			s.WriteString(m.selectedItemStyle.Render(m.selectedPrefix + item.text))
		case isCurrent:
			s.WriteString(m.selectedItemStyle.Render(m.unselectedPrefix + item.text))
		default:
			s.WriteString(m.itemStyle.Render(m.unselectedPrefix + item.text))
		}

		if i != m.height {
			s.WriteRune('\n')
		}
	}

	if m.paginator.TotalPages > 1 {
		s.WriteString(strings.Repeat("\n", m.height-m.paginator.ItemsOnPage(len(m.items))+1))
		s.WriteString(m.paginatorStyle.Render(" " + m.paginator.View()))
	}

	var parts []string
	if m.header != "" {
		parts = append(parts, m.headerStyle.Render(m.header))
	}
	parts = append(parts, s.String())
	if m.showHelp {
		parts = append(parts, "", m.help.View(m.keymap))
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func clamp(x, low, high int) int {
	if x < low {
		return low
	}
	if x > high {
		return high
	}
	return x
}
