package markdown

import (
	"html"
	"strings"
)

func tableMarkdown(table Node) string {
	rows := tableRows(table)
	if len(rows) == 0 {
		return ""
	}
	header := rows[0]
	lines := []string{"| " + strings.Join(markdownCells(header), " | ") + " |"}
	aligns := make([]string, len(header.Children))
	for i, cell := range header.Children {
		switch cell.Align {
		case "right":
			aligns[i] = "---:"
		case "center":
			aligns[i] = ":---:"
		default:
			aligns[i] = "---"
		}
	}
	lines = append(lines, "| "+strings.Join(aligns, " | ")+" |")
	for _, row := range rows[1:] {
		lines = append(lines, "| "+strings.Join(markdownCells(row), " | ")+" |")
	}
	return strings.Join(lines, "\n")
}

func markdownCells(row Node) []string {
	cells := make([]string, 0, len(row.Children))
	for _, cell := range row.Children {
		text := strings.ReplaceAll(cell.Markdown(), "|", `\|`)
		cells = append(cells, strings.TrimSpace(text))
	}
	return cells
}

func tableString(table Node, includeHeader bool) string {
	rows := tableRows(table)
	if !includeHeader && len(rows) > 0 {
		rows = rows[1:]
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, strings.Join(markdownCells(row), "\t"))
	}
	return strings.Join(lines, "\n")
}

func tableHTML(table Node) string {
	rows := tableRows(table)
	if len(rows) == 0 {
		return "<table></table>"
	}
	var b strings.Builder
	b.WriteString("<table><thead>")
	b.WriteString(rowHTML(rows[0], "th"))
	b.WriteString("</thead>")
	if len(rows) > 1 {
		b.WriteString("<tbody>")
		for _, row := range rows[1:] {
			b.WriteString(rowHTML(row, "td"))
		}
		b.WriteString("</tbody>")
	}
	b.WriteString("</table>")
	return b.String()
}

func rowHTML(row Node, cellTag string) string {
	var b strings.Builder
	b.WriteString("<tr>")
	for _, cell := range row.Children {
		align := ""
		if cell.Align != "" && cell.Align != "none" {
			align = ` style="text-align:` + html.EscapeString(cell.Align) + `"`
		}
		b.WriteString("<" + cellTag + align + ">" + joinNodeHTML(cell.Children, "") + "</" + cellTag + ">")
	}
	b.WriteString("</tr>")
	return b.String()
}

func tableRows(table Node) []Node {
	rows := make([]Node, 0, len(table.Children))
	for _, row := range table.Children {
		if row.Kind == "table_row" || row.Kind == "table_header" {
			rows = append(rows, row)
		}
	}
	return rows
}
