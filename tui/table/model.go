// Package table provides a terminal table state usable from either Bubble Tea version.
package table

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/flanksource/clicky/api"
)

type Model struct {
	Table        api.TextTable
	IDs          []string
	Selected     string
	Detail       bool
	DetailOffset int
	// Width and Height are the viewport in terminal cells; see SetSize.
	Width, Height int
}

// SetSize records the viewport, for example from a tea.WindowSizeMsg, and
// keeps the detail offset within the content it now shows.
func (m *Model) SetSize(width, height int) {
	m.Width, m.Height = width, height
	m.DetailOffset = min(m.DetailOffset, m.maxDetailOffset())
}

func (m *Model) Set(data api.TextTable, ids []string) error {
	if len(data.Rows) != len(ids) {
		return fmt.Errorf("table has %d rows and %d identities", len(data.Rows), len(ids))
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] {
			return fmt.Errorf("table row identity %q is empty or duplicated", id)
		}
		seen[id] = true
	}
	if m.Detail {
		return nil
	}
	m.Table, m.IDs = data, ids
	if !seen[m.Selected] {
		m.Selected = ""
		if len(ids) > 0 {
			m.Selected = ids[0]
		}
	}
	return nil
}

func (m *Model) Key(key string) bool {
	index := m.Index()
	switch key {
	case "enter":
		m.Detail = !m.Detail
		m.DetailOffset = 0
	case "esc":
		m.Detail = false
	case "up", "k", "pgup":
		if m.Detail {
			m.DetailOffset = max(0, m.DetailOffset-1)
		} else if index > 0 {
			m.Selected = m.IDs[index-1]
		}
	case "down", "j", "pgdown":
		if m.Detail {
			m.DetailOffset = min(m.DetailOffset+1, m.maxDetailOffset())
		} else if index+1 < len(m.IDs) {
			m.Selected = m.IDs[index+1]
		}
	default:
		return false
	}
	return true
}

func (m Model) Index() int {
	for i, id := range m.IDs {
		if id == m.Selected {
			return i
		}
	}
	return -1
}

func (m Model) View() string {
	width, height := m.Width, m.Height
	if width < 1 || height < 1 {
		return ""
	}
	if m.Detail {
		return m.detailView()
	}
	columns, widths := m.layout(width - 2)
	line := func(row api.TableRow, header bool) string {
		parts := []string{}
		for i, column := range columns {
			text := column.Label
			if !header {
				text = row[column.Name].ANSI()
			}
			parts = append(parts, fit(text, widths[i]))
		}
		// Space left after the last column pads the line, not the column, so
		// a column never renders wider than its MaxWidth.
		used := len(parts) - 1
		for _, w := range widths {
			used += w
		}
		return strings.Join(parts, " ") + strings.Repeat(" ", max(0, width-2-used))
	}
	lines := []string{"  " + line(nil, true)}
	start := max(0, m.Index()-height+2)
	for i := start; i < min(len(m.Table.Rows), start+height-1); i++ {
		prefix := "  "
		if i == m.Index() {
			prefix = "› "
		}
		lines = append(lines, prefix+line(m.Table.Rows[i], false))
	}
	if len(m.Table.Rows) == 0 && height > 1 {
		lines = append(lines, fit("No rows match this sample.", width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) layout(width int) ([]api.PrettyField, []int) {
	columns := []api.PrettyField{}
	widths := []int{}
	remaining := width
	for index, column := range m.Table.Columns {
		if column.DefaultHidden {
			continue
		}
		maximum := 24
		if index == 0 {
			maximum = max(maximum, width/3)
		}
		if column.MaxWidth > 0 {
			maximum = min(maximum, column.MaxWidth)
		}
		wanted := min(maximum, max(8, ansi.StringWidth(column.Label)))
		for _, row := range m.Table.Rows {
			wanted = max(wanted, min(maximum, ansi.StringWidth(row[column.Name].String())))
		}
		if remaining < 5 {
			break
		}
		wanted = min(wanted, remaining)
		columns = append(columns, column)
		widths = append(widths, wanted)
		remaining -= wanted + 1
	}
	return columns, widths
}

// detailLines is the selected row's detail wrapped to the viewport width, or
// nil when the row has none.
func (m Model) detailLines() []string {
	index := m.Index()
	if m.Width < 1 || index < 0 || index >= len(m.Table.RowDetail) || m.Table.RowDetail[index] == nil {
		return nil
	}
	return strings.Split(ansi.Wrap(m.Table.RowDetail[index].ANSI(), m.Width, ""), "\n")
}

func (m Model) maxDetailOffset() int {
	return max(0, len(m.detailLines())-m.Height)
}

func (m Model) detailView() string {
	lines := m.detailLines()
	if lines == nil {
		return "No detail available"
	}
	start := min(m.DetailOffset, m.maxDetailOffset())
	return strings.Join(lines[start:min(len(lines), start+m.Height)], "\n")
}

func fit(value string, width int) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = ansi.Truncate(value, max(0, width), "…")
	return value + strings.Repeat(" ", max(0, width-ansi.StringWidth(value)))
}
