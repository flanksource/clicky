package api

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const plainTableColumnGap = "  "

// plainTable renders the table as aligned columns under a header rule, without
// borders or a table-layout dependency. Columns are sized by display width
// (ANSI escapes excluded) and a multi-line cell spans several physical rows.
func (t TextTable) plainTable(withColors bool) string {
	if len(t.Headers) == 0 {
		return ""
	}
	filtered := t.WithoutEmptyColumns()
	if len(filtered.Headers) == 0 {
		return ""
	}
	headers, rows := filtered.cellStrings(withColors)

	cells := make([][][]string, 0, len(rows)+1)
	cells = append(cells, splitPlainCells(headers))
	for _, row := range rows {
		cells = append(cells, splitPlainCells(row))
	}

	widths := make([]int, len(headers))
	for _, row := range cells {
		for col, lines := range row {
			for _, line := range lines {
				widths[col] = max(widths[col], ansi.StringWidth(line))
			}
		}
	}

	rule := make([]string, len(widths))
	for col, width := range widths {
		rule[col] = strings.Repeat("─", width)
	}

	out := plainTableRow(cells[0], widths)
	out = append(out, strings.Join(rule, plainTableColumnGap))
	for _, row := range cells[1:] {
		out = append(out, plainTableRow(row, widths)...)
	}
	return strings.Join(out, "\n")
}

func splitPlainCells(row []string) [][]string {
	lines := make([][]string, len(row))
	for col, cell := range row {
		lines[col] = strings.Split(strings.ReplaceAll(cell, "\r\n", "\n"), "\n")
	}
	return lines
}

// plainTableRow lays one logical row out as physical lines, padding every
// column but the last to its width and trimming trailing padding.
func plainTableRow(row [][]string, widths []int) []string {
	height := 0
	for _, lines := range row {
		height = max(height, len(lines))
	}
	out := make([]string, height)
	for i := range out {
		var line strings.Builder
		for col, lines := range row {
			if col > 0 {
				line.WriteString(plainTableColumnGap)
			}
			var text string
			if i < len(lines) {
				text = lines[i]
			}
			line.WriteString(text)
			if col < len(row)-1 {
				line.WriteString(strings.Repeat(" ", widths[col]-ansi.StringWidth(text)))
			}
		}
		out[i] = strings.TrimRight(line.String(), " ")
	}
	return out
}
