package api

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/flanksource/clicky/api/tailwind"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/samber/lo"
)

func (t TextTable) HTML() string {
	fmt.Fprintf(os.Stderr, "DEBUG table.HTML(): table has %d headers, %d rows, %d columns\n", len(t.Headers), len(t.Rows), len(t.Columns))
	result := t.render(renderer.NewHTML(), TransformerHTML)
	fmt.Fprintf(os.Stderr, "DEBUG table.HTML(): result length = %d\n", len(result))
	return result
}

func (t TextTable) String() string {
	return t.renderLipgloss(false, TransformerString)
}

func (t TextTable) ANSI() string {
	return "\n" + t.renderLipgloss(true, TransformerANSI)
}

func (t TextTable) Markdown() string {
	return "\n" + t.render(renderer.NewMarkdown(), TransformerMarkdown)
}

var TransformerANSI TextTransformer = func(t Textable) string {
	return t.ANSI()
}

var TransformerString TextTransformer = func(t Textable) string {
	return t.String()
}
var TransformerHTML TextTransformer = func(t Textable) string {
	return t.HTML()
}

var TransformerMarkdown TextTransformer = func(t Textable) string {
	return t.Markdown()
}

type TextTransformer func(t Textable) string

func (t *TextTable) render(renderer tw.Renderer, transform TextTransformer) string {
	if len(t.Headers) == 0 {
		return ""
	}

	// Create buffer to capture table output
	var buf bytes.Buffer

	width := GetTerminalWidth()

	// Create tablewriter instance with word wrapping enabled
	// Set reasonable table max width to enable wrapping (this is distributed across columns)
	table := tablewriter.NewTable(&buf,
		tablewriter.WithRowAutoWrap(tw.WrapBreak),
		tablewriter.WithHeaderAutoFormat(tw.On),
		tablewriter.WithMaxWidth(width),
		tablewriter.WithBehavior(tw.Behavior{AutoHide: tw.On}),
		tablewriter.WithRenderer(renderer),
	)

	table.Header(lo.ToAnySlice(t.Headers.AsString())...)

	for _, row := range t.Rows {
		values := []any{}

		for i, header := range t.Headers {
			// Use field name from FieldNames if available, otherwise fall back to header
			var fieldName string
			if i < len(t.FieldNames) && t.FieldNames[i] != "" {
				fieldName = t.FieldNames[i]
			} else {
				fieldName = transform(header)
			}

			cell, ok := row[fieldName]
			if !ok {
				values = append(values, "")
				continue
			}
			values = append(values, transform(cell))
		}

		if err := table.Append(values...); err != nil {
			return err.Error()
		}
	}

	// Render the table
	if err := table.Render(); err != nil {
		return err.Error()
	}

	return buf.String()
}

func (t *TextTable) renderLipgloss(withColors bool, transform TextTransformer) string {
	if len(t.Headers) == 0 {
		return ""
	}

	width := GetTerminalWidth()

	// Build header strings
	headers := make([]string, len(t.Headers))
	for i, h := range t.Headers {
		headers[i] = transform(h)
	}

	// Build rows
	rows := make([][]string, len(t.Rows))
	for rowIdx, row := range t.Rows {
		rowData := make([]string, len(t.Headers))
		for colIdx, header := range t.Headers {
			// Use field name from FieldNames if available, otherwise fall back to header
			var fieldName string
			if colIdx < len(t.FieldNames) && t.FieldNames[colIdx] != "" {
				fieldName = t.FieldNames[colIdx]
			} else {
				fieldName = transform(header)
			}

			cell, ok := row[fieldName]
			if !ok {
				rowData[colIdx] = ""
				continue
			}
			rowData[colIdx] = transform(cell)
		}
		rows[rowIdx] = rowData
	}

	// Create lipgloss table
	tbl := table.New().
		Headers(headers...).
		Rows(rows...).
		Width(width)

	// Apply styling if colors are enabled
	if withColors {
		tbl = tbl.StyleFunc(func(row, col int) lipgloss.Style {
			// row -1 is the header
			if row == -1 {
				return lipgloss.NewStyle().Bold(true)
			}

			// Get the cell value to check for styling
			if row < len(t.Rows) && col < len(t.Headers) {
				var fieldName string
				if col < len(t.FieldNames) && t.FieldNames[col] != "" {
					fieldName = t.FieldNames[col]
				} else {
					fieldName = TransformerString(t.Headers[col])
				}

				if cell, ok := t.Rows[row][fieldName]; ok {
					// Check if the Textable is a Text type and has a Style
					if textCell, isText := cell.Textable.(*Text); isText && textCell.Style != "" {
						return parseTailwindToLipgloss(textCell.Style)
					}
				}
			}

			return lipgloss.NewStyle()
		})
	}

	return tbl.String()
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
