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
			m.DetailOffset++
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

func (m Model) View(width, height int) string {
	if width < 1 || height < 1 {
		return ""
	}
	if m.Detail {
		return m.detailView(width, height)
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
		return strings.Join(parts, " ")
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
		lines = append(lines, "No rows match this sample.")
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
	if len(widths) > 0 {
		widths[len(widths)-1] += max(0, remaining+1)
	}
	return columns, widths
}

func (m Model) detailView(width, height int) string {
	index := m.Index()
	if index < 0 || index >= len(m.Table.RowDetail) || m.Table.RowDetail[index] == nil {
		return "No detail available"
	}
	lines := strings.Split(ansi.Wrap(m.Table.RowDetail[index].ANSI(), width, ""), "\n")
	start := min(m.DetailOffset, max(0, len(lines)-height))
	return strings.Join(lines[start:min(len(lines), start+height)], "\n")
}

func fit(value string, width int) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = ansi.Truncate(value, max(0, width), "…")
	return value + strings.Repeat(" ", max(0, width-ansi.StringWidth(value)))
}
