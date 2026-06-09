package docs

import (
	"fmt"
	"strings"
)

// mdBuilder is a small structured-markdown writer. It exists because
// api.Text.Markdown() renders inline styling but not document structure
// (headings, tables, fenced code blocks), which is what a docs page needs.
type mdBuilder struct {
	b strings.Builder
}

func (m *mdBuilder) heading(level int, text string) *mdBuilder {
	if level < 1 {
		level = 1
	}
	m.b.WriteString(strings.Repeat("#", level))
	m.b.WriteByte(' ')
	m.b.WriteString(text)
	m.b.WriteString("\n\n")
	return m
}

func (m *mdBuilder) para(text string) *mdBuilder {
	if text == "" {
		return m
	}
	m.b.WriteString(text)
	m.b.WriteString("\n\n")
	return m
}

func (m *mdBuilder) line(text string) *mdBuilder {
	m.b.WriteString(text)
	m.b.WriteByte('\n')
	return m
}

func (m *mdBuilder) blank() *mdBuilder {
	m.b.WriteByte('\n')
	return m
}

func (m *mdBuilder) codeBlock(lang, body string) *mdBuilder {
	m.b.WriteString("```")
	m.b.WriteString(lang)
	m.b.WriteByte('\n')
	m.b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		m.b.WriteByte('\n')
	}
	m.b.WriteString("```\n\n")
	return m
}

// table writes a GitHub-flavored markdown table. Rows with fewer cells than
// headers are padded; pipes in cells are escaped.
func (m *mdBuilder) table(headers []string, rows [][]string) *mdBuilder {
	if len(headers) == 0 {
		return m
	}
	m.b.WriteString("| " + strings.Join(escapeCells(headers), " | ") + " |\n")
	sep := make([]string, len(headers))
	for i := range sep {
		sep[i] = "---"
	}
	m.b.WriteString("| " + strings.Join(sep, " | ") + " |\n")
	for _, row := range rows {
		cells := make([]string, len(headers))
		for i := range headers {
			if i < len(row) {
				cells[i] = row[i]
			}
		}
		m.b.WriteString("| " + strings.Join(escapeCells(cells), " | ") + " |\n")
	}
	m.b.WriteByte('\n')
	return m
}

func (m *mdBuilder) String() string {
	return m.b.String()
}

func escapeCells(cells []string) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		c = strings.ReplaceAll(c, "|", "\\|")
		out[i] = strings.ReplaceAll(c, "\n", " ")
	}
	return out
}

func code(s string) string {
	if s == "" {
		return ""
	}
	return "`" + s + "`"
}

func boolMark(b bool) string {
	if b {
		return "✓"
	}
	return ""
}

func defaultStr(v interface{}) string {
	if v == nil {
		return ""
	}
	s := fmt.Sprintf("%v", v)
	if s == "" {
		return ""
	}
	return code(s)
}
