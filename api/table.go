package api

import "strings"

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

func (t TextTable) Markdown() string {
	return t.MarkdownWithOptions(MarkdownOptions{})
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

// cellStrings renders the header and every row cell for a terminal table, as
// ANSI when withColors is set and as plain text otherwise.
func (t *TextTable) cellStrings(withColors bool) (headers []string, rows [][]string) {
	render := Textable.String
	if withColors {
		render = Textable.ANSI
	}
	headers = make([]string, len(t.Headers))
	for i, header := range t.Headers {
		headers[i] = render(header)
	}
	rows = make([][]string, len(t.Rows))
	for rowIdx, row := range t.Rows {
		rows[rowIdx] = make([]string, len(t.Headers))
		for colIdx := range t.Headers {
			rows[rowIdx][colIdx] = render(t.getCellValue(row, colIdx))
		}
	}
	return headers, rows
}
