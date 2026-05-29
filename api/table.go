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

// WithoutEmptyColumns returns a copy of the table with columns removed where
// every row has an empty value. Used by display renderers (ANSI, HTML, PDF)
// but not by data formats (CSV, Excel, JSON, YAML).
func (t TextTable) WithoutEmptyColumns() TextTable {
	if len(t.Headers) == 0 || len(t.Rows) == 0 {
		return t
	}

	nonEmpty := make([]bool, len(t.Headers))
	for _, row := range t.Rows {
		for i := range t.Headers {
			if nonEmpty[i] {
				continue
			}
			cell := t.getCellValue(row, i)
			if strings.TrimSpace(cell.String()) != "" {
				nonEmpty[i] = true
			}
		}
	}

	out := TextTable{Interactive: t.Interactive, RowDetail: t.RowDetail}
	for i, keep := range nonEmpty {
		if !keep {
			continue
		}
		out.Headers = append(out.Headers, t.Headers[i])
		if i < len(t.FieldNames) {
			out.FieldNames = append(out.FieldNames, t.FieldNames[i])
		}
		if i < len(t.Columns) {
			out.Columns = append(out.Columns, t.Columns[i])
		}
	}
	for _, row := range t.Rows {
		newRow := TableRow{}
		for i, keep := range nonEmpty {
			if !keep {
				continue
			}
			var fieldName string
			if i < len(t.FieldNames) && t.FieldNames[i] != "" {
				fieldName = t.FieldNames[i]
			} else {
				fieldName = t.Headers[i].String()
			}
			if val, ok := row[fieldName]; ok {
				newRow[fieldName] = val
			}
		}
		out.Rows = append(out.Rows, newRow)
	}
	return out
}

// hasAnyRowDetail returns true if any row has non-nil detail content.
func (t TextTable) hasAnyRowDetail() bool {
	for _, d := range t.RowDetail {
		if d != nil {
			return true
		}
	}
	return false
}

// getRowDetail returns the detail content for a row, or nil.
func (t TextTable) getRowDetail(rowIdx int) Textable {
	if rowIdx < len(t.RowDetail) {
		return t.RowDetail[rowIdx]
	}
	return nil
}

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
	table = table.WithoutEmptyColumns()

	hasDetail := table.hasAnyRowDetail()
	colCount := len(table.Headers)
	if hasDetail {
		colCount++
	}

	var result strings.Builder
	result.WriteString(`<table class="inline-table text-xs border-collapse border border-gray-300">`)

	// Headers
	result.WriteString("<thead><tr>")
	if hasDetail {
		result.WriteString(`<th class="border border-gray-300 px-2 py-1 bg-gray-100 w-6"></th>`)
	}
	for _, header := range table.Headers {
		fmt.Fprintf(&result, `<th class="border border-gray-300 px-2 py-1 bg-gray-100 font-semibold">%s</th>`,
			html.EscapeString(header.String()))
	}
	result.WriteString("</tr></thead>")

	// Rows
	for i, row := range table.Rows {
		detail := table.getRowDetail(i)

		if detail != nil {
			result.WriteString(`<tbody x-data="{ open: false }">`)
			result.WriteString(`<tr class="cursor-pointer" @click="open = !open">`)
			result.WriteString(`<td class="border border-gray-300 px-2 py-1 text-center">`)
			result.WriteString(`<iconify-icon x-show="!open" icon="codicon:chevron-right"></iconify-icon>`)
			result.WriteString(`<iconify-icon x-show="open" style="display:none" icon="codicon:chevron-down"></iconify-icon>`)
			result.WriteString(`</td>`)
		} else {
			result.WriteString(`<tbody>`)
			result.WriteString("<tr>")
			if hasDetail {
				result.WriteString(`<td class="border border-gray-300 px-2 py-1"></td>`)
			}
		}

		for _, header := range table.Headers {
			cellValue, ok := row[header.String()]
			var cellHTML string
			if ok {
				cellHTML = cellValue.HTML()
			}
			fmt.Fprintf(&result, `<td class="border border-gray-300 px-2 py-1">%s</td>`, cellHTML)
		}
		result.WriteString("</tr>")

		if detail != nil {
			if sp, ok := detail.(StaticHTMLProvider); ok {
				result.WriteString(`<tr><td colspan="`)
				fmt.Fprintf(&result, `%d" class="border border-gray-300 px-3 py-2 bg-gray-50 whitespace-normal break-words max-w-0">`, colCount)
				result.WriteString(sp.StaticHTML())
				result.WriteString(`</td></tr>`)
			} else {
				result.WriteString(`<template x-if="open"><tr>`)
				fmt.Fprintf(&result, `<td colspan="%d" class="border border-gray-300 px-3 py-2 bg-gray-50 whitespace-normal break-words max-w-0">`, colCount)
				result.WriteString(detail.HTML())
				result.WriteString(`</td></tr></template>`)
			}
		}
		result.WriteString(`</tbody>`)
	}
	result.WriteString("</table>")

	return result.String()
}

// StaticHTML renders a pure HTML table without JavaScript (Grid.js).
// This is suitable for PDF output where JavaScript may not execute.
func (table TextTable) StaticHTML() string {
	if len(table.Rows) == 0 {
		return `<p class="text-gray-500 text-center py-8">No data available</p>`
	}
	table = table.WithoutEmptyColumns()

	columns := table.Columns
	if len(columns) == 0 && len(table.Headers) > 0 {
		columns = make([]PrettyField, 0, len(table.Headers))
		for i, header := range table.Headers {
			name := header.String()
			if i < len(table.FieldNames) && table.FieldNames[i] != "" {
				name = table.FieldNames[i]
			}
			columns = append(columns, PrettyField{
				Name:  name,
				Label: header.String(),
			})
		}
	}

	if len(columns) == 0 {
		return `<p class="text-red-500 text-center py-8">Table has no columns defined</p>`
	}

	hasDetail := table.hasAnyRowDetail()
	colCount := len(columns)
	if hasDetail {
		colCount++ // extra column for chevron
	}

	var result strings.Builder
	result.WriteString(`<div class="overflow-x-auto px-6 py-4">`)
	result.WriteString(`<table class="w-full border-collapse text-sm">`)

	// Headers
	result.WriteString(`<thead><tr class="bg-gray-100">`)
	if hasDetail {
		result.WriteString(`<th class="border border-gray-300 px-3 py-2 w-8"></th>`)
	}
	for _, col := range columns {
		headerLabel := col.Label
		if headerLabel == "" {
			headerLabel = PrettifyFieldName(col.Name)
		}
		fmt.Fprintf(&result, `<th class="border border-gray-300 px-3 py-2 text-left font-semibold">%s</th>`,
			html.EscapeString(headerLabel))
	}
	result.WriteString("</tr></thead>")

	// Rows - use separate <tbody> per expandable row pair for Alpine.js scoping
	for i, row := range table.Rows {
		detail := table.getRowDetail(i)
		rowClass := ""
		if i%2 == 1 {
			rowClass = " bg-gray-50"
		}

		if detail != nil {
			result.WriteString(`<tbody x-data="{ open: false }">`)
			fmt.Fprintf(&result, `<tr class="cursor-pointer hover:bg-gray-100%s" @click="open = !open">`, rowClass)
			result.WriteString(`<td class="border border-gray-300 px-3 py-2 text-center">`)
			result.WriteString(`<iconify-icon x-show="!open" icon="codicon:chevron-right"></iconify-icon>`)
			result.WriteString(`<iconify-icon x-show="open" style="display:none" icon="codicon:chevron-down"></iconify-icon>`)
			result.WriteString(`</td>`)
		} else {
			result.WriteString(`<tbody>`)
			if hasDetail {
				fmt.Fprintf(&result, `<tr class="%s">`, rowClass)
				result.WriteString(`<td class="border border-gray-300 px-3 py-2"></td>`)
			} else {
				fmt.Fprintf(&result, `<tr class="%s">`, rowClass)
			}
		}

		for _, col := range columns {
			fieldValue, exists := row[col.Name]
			var cellContent string
			if exists {
				cellContent = fieldValue.HTML()
			}
			fmt.Fprintf(&result, `<td class="border border-gray-300 px-3 py-2">%s</td>`, cellContent)
		}
		result.WriteString("</tr>")

		// Detail row
		if detail != nil {
			result.WriteString(`<template x-if="open"><tr>`)
			fmt.Fprintf(&result, `<td colspan="%d" class="border border-gray-300 px-6 py-4 bg-gray-50 whitespace-normal break-words max-w-0">`, colCount)
			result.WriteString(detail.HTML())
			result.WriteString(`</td></tr></template>`)
		}
		result.WriteString(`</tbody>`)
	}
	result.WriteString("</table></div>")

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
	table = table.WithoutEmptyColumns()

	// Use table's embedded Columns if available, otherwise derive from headers.
	columns := table.Columns
	if len(columns) == 0 && len(table.Headers) > 0 {
		columns = make([]PrettyField, 0, len(table.Headers))
		for i, header := range table.Headers {
			name := header.String()
			if i < len(table.FieldNames) && table.FieldNames[i] != "" {
				name = table.FieldNames[i]
			}
			columns = append(columns, PrettyField{
				Name:  name,
				Label: header.String(),
			})
		}
	}

	if len(columns) == 0 {
		return "            <p class=\"text-red-500 text-center py-8\">Table has no columns defined</p>"
	}

	// When rows have detail content, use a custom HTML table with Alpine.js
	// instead of Grid.js (which doesn't support expandable rows)
	if table.hasAnyRowDetail() {
		return table.htmlWithDetail(columns)
	}

	var result strings.Builder

	tableID := fmt.Sprintf("gridjs-table-%d", tableIDCounter.Add(1))

	// Create a div for Grid.js to mount
	fmt.Fprintf(&result, "            <div id=\"%s\"></div>\n", tableID)

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
		fmt.Fprintf(&result, "     { name: %s, sort: true, formatter: (cell) => gridjs.html(cell) }", jsonEscape(headerLabel))
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

	fmt.Fprintf(&result, "                    }).render(document.getElementById('%s')).then(() => {\n", tableID)
	result.WriteString(" // Reinitialize tooltips after Grid.js renders the table\n")
	result.WriteString(" if (typeof initTooltips === 'function') {\n")
	result.WriteString("     initTooltips();\n")
	result.WriteString(" }\n")
	result.WriteString("                    });\n")
	result.WriteString("                });\n")
	result.WriteString("            </script>\n")

	return result.String()
}

// htmlWithDetail renders an HTML table with expandable row details using Alpine.js.
func (table TextTable) htmlWithDetail(columns []PrettyField) string {
	colCount := len(columns) + 1 // +1 for chevron column

	var result strings.Builder
	result.WriteString(`<div class="overflow-x-auto px-6 py-4">`)
	result.WriteString(`<table class="w-full border-collapse text-sm">`)

	// Headers
	result.WriteString(`<thead><tr class="bg-gray-100">`)
	result.WriteString(`<th class="border border-gray-300 px-3 py-2 w-8"></th>`)
	for _, col := range columns {
		headerLabel := col.Label
		if headerLabel == "" {
			headerLabel = PrettifyFieldName(col.Name)
		}
		fmt.Fprintf(&result, `<th class="border border-gray-300 px-3 py-2 text-left font-semibold">%s</th>`,
			html.EscapeString(headerLabel))
	}
	result.WriteString("</tr></thead>")

	// Rows
	for i, row := range table.Rows {
		detail := table.getRowDetail(i)
		rowClass := ""
		if i%2 == 1 {
			rowClass = " bg-gray-50"
		}

		if detail != nil {
			result.WriteString(`<tbody x-data="{ open: false }">`)
			fmt.Fprintf(&result, `<tr class="cursor-pointer hover:bg-gray-100%s" @click="open = !open">`, rowClass)
			result.WriteString(`<td class="border border-gray-300 px-3 py-2 text-center">`)
			result.WriteString(`<iconify-icon x-show="!open" icon="codicon:chevron-right"></iconify-icon>`)
			result.WriteString(`<iconify-icon x-show="open" style="display:none" icon="codicon:chevron-down"></iconify-icon>`)
			result.WriteString(`</td>`)
		} else {
			result.WriteString(`<tbody>`)
			fmt.Fprintf(&result, `<tr class="%s">`, rowClass)
			result.WriteString(`<td class="border border-gray-300 px-3 py-2"></td>`)
		}

		for _, col := range columns {
			fieldValue, exists := row[col.Name]
			var cellContent string
			if exists {
				cellContent = fieldValue.HTML()
			}
			fmt.Fprintf(&result, `<td class="border border-gray-300 px-3 py-2">%s</td>`, cellContent)
		}
		result.WriteString("</tr>")

		if detail != nil {
			result.WriteString(`<template x-if="open"><tr>`)
			fmt.Fprintf(&result, `<td colspan="%d" class="border border-gray-300 px-6 py-4 bg-gray-50 whitespace-normal break-words max-w-0">`, colCount)
			result.WriteString(detail.HTML())
			result.WriteString(`</td></tr></template>`)
		}
		result.WriteString(`</tbody>`)
	}
	result.WriteString("</table></div>")

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
				fieldName = header.String()
			}

			cell, ok := row[fieldName]
			if !ok {
				values = append(values, "")
				continue
			}
			values = append(values, escapeMarkdownPipes(TransformerMarkdown(cell)))
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

// escapeMarkdownPipes escapes a literal pipe so a cell value does not break the
// GFM table layout. The tablewriter markdown renderer does not escape pipes, so
// callers that put `|` in cell content rely on this.
func escapeMarkdownPipes(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

type TextTransformer func(t Textable) string

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
	filtered := t.WithoutEmptyColumns()
	t = &filtered
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
