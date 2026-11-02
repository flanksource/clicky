//go:build !wasm

package api

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/x/ansi"
	"github.com/flanksource/clicky/api/tailwind"
)

func (t TextTable) String() string {
	return t.renderLipgloss(false)
}

func (t TextTable) ANSI() string {
	return "\n" + t.renderLipgloss(true)
}

func (t *TextTable) renderLipgloss(withColors bool) string {
	if len(t.Headers) == 0 {
		return ""
	}
	filtered := t.WithoutEmptyColumns()
	t = &filtered
	if len(t.Headers) == 0 {
		return ""
	}

	borderWidth := len(t.Headers) + 1
	available := GetTerminalWidth() - borderWidth

	// Calculate max width per column using .String() for accurate measurement.
	// No column is allocated more than the available width, so measuring stops
	// there: a cell can hold hundreds of kilobytes that end up as a few
	// characters, and measuring all of it dominated rendering such a table.
	widthLimit := max(available, columnMinWidth)
	columnWidths := make([]int, len(t.Headers))

	// Measure headers
	for i, header := range t.Headers {
		columnWidths[i] = cappedWidth(header.String(), widthLimit)
	}

	// Measure all row cells to find max width per column
	for _, row := range t.Rows {
		for colIdx := range t.Headers {
			cell := t.getCellValue(row, colIdx)
			width := cappedWidth(cell.String(), widthLimit)
			if width > columnWidths[colIdx] {
				columnWidths[colIdx] = width
			}
		}
	}

	headers, rows := t.cellStrings(withColors)

	// Size the columns here rather than letting lipgloss do it. Lipgloss shrinks
	// whichever column is currently widest, one character at a time, with no
	// floor -- so a narrow column keeps its width only by accident, and a table
	// that fits gets padded out to fill the terminal.
	widths := allocateColumnWidths(columnWidths, available)

	tableWidth := borderWidth
	for _, width := range widths {
		tableWidth += width
	}

	rowHeights := make([]int, len(rows))
	for row, cells := range rows {
		for _, cell := range cells {
			rowHeights[row] = max(rowHeights[row], strings.Count(strings.ReplaceAll(cell, "\r\n", "\n"), "\n")+1)
		}
	}

	// Lipgloss measures every cell and truncates it to its column width times
	// its row height; hand it only the part of each cell that can survive that.
	for row, cells := range rows {
		for col, cell := range cells {
			cells[col] = clipCell(cell, widths[col]*rowHeights[row])
		}
	}

	// Every column is pinned below, so lipgloss's own resizing is inert as long
	// as the table width matches what we allocated -- both maxColumnWidths and
	// maxTotal honour a pinned width. Wrap(false) makes it truncate a cell that
	// still overflows instead of wrapping it onto a second line; the cell text
	// was already capped at the column's MaxWidth upstream, but the allocated
	// width can be narrower still, and only lipgloss knows the value is ANSI.
	tbl := table.New().
		Headers(headers...).
		Rows(rows...).
		Wrap(false).
		Width(tableWidth)

	tbl = tbl.StyleFunc(func(row, col int) lipgloss.Style {
		style := lipgloss.NewStyle()
		if withColors {
			if row == -1 {
				style = style.Bold(true)
			} else if row < len(t.Rows) && col < len(t.Headers) {
				cell := t.getCellValue(t.Rows[row], col)
				if textCell, isText := cell.(*Text); isText && textCell.Style != "" {
					style = parseTailwindToLipgloss(textCell.Style)
				}
			}
		}
		if col < len(widths) {
			style = style.Width(widths[col])
		}
		if row >= 0 && row < len(rowHeights) {
			style = style.Height(rowHeights[row])
		}
		return style
	})

	return tbl.String()
}

// clipCell returns a prefix of cell that ansi.Truncate, which lipgloss uses to
// cut a cell to its box, truncates to the same string as the whole cell for
// any length up to limit. It returns cell unchanged when it is short, or when
// it holds escape sequences or invalid UTF-8, whose bytes past the cut
// ansi.Truncate keeps.
func clipCell(cell string, limit int) string {
	// A grapheme is at most two cells wide, so a prefix at least limit+4 wide
	// reaches limit within graphemes that also end the same way in the whole
	// cell, wherever the prefix splits the last one.
	end := widthPrefix(cell, limit+4)
	if end == len(cell) || strings.IndexByte(cell, ansi.ESC) >= 0 || !utf8.ValidString(cell) {
		return cell
	}
	return cell[:end]
}

// cappedWidth returns lipgloss.Width(s), or limit when s is at least that wide.
func cappedWidth(s string, limit int) int {
	width := 0
	for line := range strings.SplitSeq(s, "\n") {
		// The prefix can end inside a grapheme and measure up to two cells
		// wider than the same bytes do within the line.
		if widthPrefix(line, limit+2) < len(line) {
			return limit
		}
		width = max(width, ansi.StringWidth(line))
	}
	return min(width, limit)
}

// widthPrefix returns the length of a prefix of s, ending on a rune boundary,
// that is at least width cells wide, or len(s) when no shorter one was found.
// It measures prefixes of doubling length, so it reads a bounded part of s.
func widthPrefix(s string, width int) int {
	for end := 4*max(width, 0) + 16; end < len(s); end *= 2 {
		for end > 0 && !utf8.RuneStart(s[end]) {
			end--
		}
		if ansi.StringWidth(s[:end]) >= width {
			return end
		}
	}
	return len(s)
}

// columnMinWidth is the narrowest a column may be shrunk to, leaving room for a
// character and the ellipsis truncation appends.
const columnMinWidth = 2

// allocateColumnWidths sizes each column to its content, then, if that overruns
// the available width, lowers every column to a common ceiling chosen as high as
// the budget allows.
//
// Capping the widest columns is what keeps a narrow one intact: a project name
// or a token count sits well under the ceiling and is never touched, while the
// prose column beside it gives up everything the terminal demands. Taking a
// proportional cut from every column instead would clip a 7-character project
// name to free space its neighbour has in abundance.
func allocateColumnWidths(natural []int, available int) []int {
	widths := append([]int(nil), natural...)

	total, widest := 0, 0
	for _, width := range widths {
		total += width
		widest = max(widest, width)
	}
	if total <= available {
		return widths
	}

	// Highest ceiling whose capped total still fits. Below columnMinWidth there
	// is nothing left to give, so a table with more columns than the terminal
	// can hold overflows rather than collapsing to slivers.
	low, high := columnMinWidth, widest
	for low < high {
		ceiling := (low + high + 1) / 2
		capped := 0
		for _, width := range widths {
			capped += min(width, ceiling)
		}
		if capped <= available {
			low = ceiling
		} else {
			high = ceiling - 1
		}
	}

	for i, width := range widths {
		widths[i] = min(width, low)
	}
	return widths
}

// parseTailwindToLipgloss converts a Tailwind style string to a lipgloss.Style
func parseTailwindToLipgloss(tailwindStyle string) lipgloss.Style {
	style := lipgloss.NewStyle()

	// Parse the Tailwind style string
	classes := strings.Fields(tailwindStyle)
	for _, class := range classes {
		// Handle text colors
		if strings.HasPrefix(class, "text-") {
			if color, err := tailwind.ParseTailwindColor(class); err == nil && color != "" {
				style = style.Foreground(lipgloss.Color(color))
			}
		}
		// Handle background colors
		if strings.HasPrefix(class, "bg-") {
			if color, err := tailwind.ParseTailwindColor(class); err == nil && color != "" {
				style = style.Background(lipgloss.Color(color))
			}
		}
		// Handle font weights
		switch class {
		case "bold", "font-bold", "font-semibold":
			style = style.Bold(true)
		case "italic", "font-italic":
			style = style.Italic(true)
		case "underline":
			style = style.Underline(true)
		case "strikethrough", "line-through":
			style = style.Strikethrough(true)
		}
	}

	return style
}

func (t TextTable) MarkdownWithOptions(options MarkdownOptions) string {
	return t.plainMarkdown(options)
}
