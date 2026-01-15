package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/samber/lo"

	"github.com/flanksource/clicky/api/tailwind"
)

var tableIDCounter = atomic.Int64{}

// jsonEscape properly escapes a string for use in JSON
func jsonEscape(s string) string {
	// Use Go's JSON marshaling to properly escape the string
	escaped, _ := json.Marshal(s)
	return string(escaped)
}

func (table TextTable) CompactHTML() string {
	if len(table.Rows) == 0 {
		return `<span class="text-gray-400">Empty</span>`
	}

	var result strings.Builder
	result.WriteString(`<table class="inline-table text-xs border-collapse border border-gray-300">`)

	// Headers
	result.WriteString("<thead><tr>")
	for _, header := range table.Headers {
		result.WriteString(fmt.Sprintf(`<th class="border border-gray-300 px-2 py-1 bg-gray-100 font-semibold">%s</th>`,
			html.EscapeString(header.String())))
	}
	result.WriteString("</tr></thead>")

	// Rows
	result.WriteString("<tbody>")
	for _, row := range table.Rows {
		result.WriteString("<tr>")
		for _, header := range table.Headers {
			cellValue := row[header.String()]
			result.WriteString(fmt.Sprintf(`<td class="border border-gray-300 px-2 py-1">%s</td>`,
				cellValue.HTML()))
		}
		result.WriteString("</tr>")
	}
	result.WriteString("</tbody></table>")

	return result.String()
}

func (table TextTable) PrintableHTML() string {

	return table.html(true)
}

func (table TextTable) HTML() string {
	return table.html(false)
}
func (table TextTable) html(interactive bool) string {
	if len(table.Rows) == 0 {
		return "            <p class=\"text-gray-500 text-center py-8\">No data available</p>"
	}
	if len(table.Columns) == 0 {
		return "            <p class=\"text-red-500 text-center py-8\">Table has no columns defined</p>"
	}

	// Use table's embedded Columns if available, otherwise use field.TableOptions.Columns
	columns := table.Columns

	var result strings.Builder

	tableID := fmt.Sprintf("gridjs-table-%d", tableIDCounter.Add(1))

	// Create a div for Grid.js to mount
	result.WriteString(fmt.Sprintf("            <div id=\"%s\"></div>\n", tableID))

	// Generate JavaScript to initialize Grid.js
	result.WriteString("            <script>\n")
	result.WriteString("                document.addEventListener('DOMContentLoaded', function() {\n")
	result.WriteString("                    new gridjs.Grid({\n")

	// Configure columns
	result.WriteString(" columns: [\n")
	for i, tableField := range columns {
		headerLabel := tableField.Label
		if headerLabel == "" {
			headerLabel = PrettifyFieldName(tableField.Name)
		}

		if i > 0 {
			result.WriteString(",\n")
		}

		// Format column definition with sorting and HTML rendering enabled
		result.WriteString(fmt.Sprintf("     { name: %s, sort: true, formatter: (cell) => gridjs.html(cell) }", jsonEscape(headerLabel)))
	}
	result.WriteString("\n ],\n")

	// Configure data
	result.WriteString(" data: [\n")
	for i, row := range table.Rows {
		if i > 0 {
			result.WriteString(",\n")
		}
		result.WriteString("     [")

		for j, tableField := range columns {
			if j > 0 {
				result.WriteString(", ")
			}

			fieldValue, exists := row[tableField.Name]
			var cellContent string
			if exists {
				cellContent = fieldValue.HTML()
			} else {
				cellContent = ""
			}
			result.WriteString(jsonEscape(cellContent))
		}
		result.WriteString("]")
	}
	result.WriteString("\n ],\n")

	// Configure Grid.js options
	if interactive {
		result.WriteString(" sort: true,resizable: true, search: true,\n")
	}
	result.WriteString(" pagination: false,\n")
	result.WriteString(" className: {\n")
	result.WriteString("     table: 'gridjs-table',\n")
	result.WriteString("     th: 'gridjs-th',\n")
	result.WriteString("     td: 'gridjs-td'\n")
	result.WriteString(" }\n")

	result.WriteString(fmt.Sprintf("                    }).render(document.getElementById('%s')).then(() => {\n", tableID))
	result.WriteString(" // Reinitialize tooltips after Grid.js renders the table\n")
	result.WriteString(" if (typeof initTooltips === 'function') {\n")
	result.WriteString("     initTooltips();\n")
	result.WriteString(" }\n")
	result.WriteString("                    });\n")
	result.WriteString("                });\n")
	result.WriteString("            </script>\n")

	return result.String()
}

func (t TextTable) String() string {
	return t.renderLipgloss(false)
}

func (t TextTable) ANSI() string {
	return "\n" + t.renderLipgloss(true)
}

func (t TextTable) Markdown() string {
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
		tablewriter.WithRenderer(renderer.NewMarkdown()),
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
				fieldName = TransformerMarkdown(header)
			}

			cell, ok := row[fieldName]
			if !ok {
				values = append(values, "")
				continue
			}
			values = append(values, TransformerMarkdown(cell))
		}

		if err := table.Append(values...); err != nil {
			return err.Error()
		}
	}

	// Render the table
	if err := table.Render(); err != nil {
		return err.Error()
	}

	return "\n" + buf.String()
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

// getCellValue retrieves the Textable value for a given cell in the table
func (t *TextTable) getCellValue(row TableRow, colIdx int) Textable {
	var fieldName string
	if colIdx < len(t.FieldNames) && t.FieldNames[colIdx] != "" {
		fieldName = t.FieldNames[colIdx]
	} else {
		fieldName = t.Headers[colIdx].String()
	}

	if cell, ok := row[fieldName]; ok {
		return cell
	}
	return &Text{Content: ""}
}

func (t *TextTable) renderLipgloss(withColors bool) string {
	if len(t.Headers) == 0 {
		return ""
	}

	// Calculate max width per column using .String() for accurate measurement
	columnWidths := make([]int, len(t.Headers))

	// Measure headers
	for i, header := range t.Headers {
		columnWidths[i] = len(header.String())
	}

	// Measure all row cells to find max width per column
	for _, row := range t.Rows {
		for colIdx := range t.Headers {
			cell := t.getCellValue(row, colIdx)
			width := len(cell.String())
			if width > columnWidths[colIdx] {
				columnWidths[colIdx] = width
			}
		}
	}

	// Build header strings for display using appropriate method
	headers := make([]string, len(t.Headers))
	for i, h := range t.Headers {
		if withColors {
			headers[i] = h.ANSI()
		} else {
			headers[i] = h.String()
		}
	}

	// Build row data for display
	rows := make([][]string, len(t.Rows))
	for rowIdx, row := range t.Rows {
		rowData := make([]string, len(t.Headers))
		for colIdx := range t.Headers {
			cell := t.getCellValue(row, colIdx)
			if withColors {
				rowData[colIdx] = cell.ANSI()
			} else {
				rowData[colIdx] = cell.String()
			}
		}
		rows[rowIdx] = rowData
	}

	// Calculate total width needed for table
	totalWidth := GetTerminalWidth()

	// Create lipgloss table with calculated dimensions
	tbl := table.New().
		Headers(headers...).
		Rows(rows...).
		Width(totalWidth)

	// Apply styling if colors are enabled
	if withColors {
		tbl = tbl.StyleFunc(func(row, col int) lipgloss.Style {
			// row -1 is the header
			if row == -1 {
				return lipgloss.NewStyle().Bold(true)
			}

			// Get the cell value to check for styling
			if row < len(t.Rows) && col < len(t.Headers) {
				cell := t.getCellValue(t.Rows[row], col)

				// Check if the Textable is a Text type and has a Style
				if textCell, isText := cell.(*Text); isText && textCell.Style != "" {
					return parseTailwindToLipgloss(textCell.Style)
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
