package api

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// plainMarkdown renders full, unpadded cells identically in native and WASM builds.
func (t TextTable) plainMarkdown(options MarkdownOptions) string {
	options.Unbounded = true
	if len(t.Headers) == 0 {
		return ""
	}
	headers := make([]string, len(t.Headers))
	for i, header := range t.Headers {
		text := RenderMarkdown(Text{Content: header.String()}, options)
		headers[i] = markdownHeaderTitle(markdownNewlinesToBr(text))
	}
	rows := make([][]string, len(t.Rows))
	for r, row := range t.Rows {
		rows[r] = make([]string, len(t.Headers))
		for i := range t.Headers {
			if cell, ok := row[t.markdownFieldName(i)]; ok {
				rows[r][i] = strings.TrimSpace(RenderMarkdown(cell, options))
			}
		}
	}
	return "\n" + plainMarkdownTable(headers, rows)
}

// plainMarkdownTable renders a GFM pipe table: a header row, a `---`
// separator row and one row per record, with no column padding.
func plainMarkdownTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}
	var b strings.Builder
	writeRow := func(cells []string) {
		b.WriteString("|")
		for _, cell := range cells {
			b.WriteString(" " + escapeMarkdownTableCell(cell) + " |")
		}
		b.WriteString("\n")
	}
	writeRow(headers)
	b.WriteString(strings.Repeat("| --- ", len(headers)) + "|\n")
	for _, row := range rows {
		writeRow(row)
	}
	return b.String()
}

// markdownHeaderTitle mirrors tablewriter's header auto-format,
// tw.Title(strings.Join(tw.SplitCamelCase(header), " ")), so native and wasm
// tables print the same header text.
func markdownHeaderTitle(header string) string {
	joined := strings.Join(splitCamelCase(header), " ")
	rs := []rune(joined)
	numericOrSpace := func(r rune) bool { return ('0' <= r && r <= '9') || r == ' ' }
	for i, r := range rs {
		switch {
		case r == '_':
			rs[i] = ' '
		case r == '.' && ((i != 0 && !numericOrSpace(rs[i-1])) || (i != len(rs)-1 && !numericOrSpace(rs[i+1]))):
			rs[i] = ' '
		}
	}
	title := strings.TrimSpace(string(rs))
	if title == "" && joined != "" {
		return " "
	}
	return strings.ToUpper(title)
}

// splitCamelCase ports tw.SplitCamelCase: runs of lower/upper/digit/other
// runes become words, an upper run hands its last rune to a following lower
// run (HTTPServer -> HTTP Server), and whitespace-only or "_" runs are dropped.
func splitCamelCase(src string) []string {
	if !utf8.ValidString(src) {
		return []string{src}
	}
	runeClass := func(r rune) int {
		switch {
		case unicode.IsLower(r):
			return 1
		case unicode.IsUpper(r):
			return 2
		case unicode.IsDigit(r):
			return 3
		}
		return 4
	}
	var runs [][]rune
	last := 0
	for _, r := range src {
		class := runeClass(r)
		if class == last {
			runs[len(runs)-1] = append(runs[len(runs)-1], r)
		} else {
			runs = append(runs, []rune{r})
		}
		last = class
	}
	for i := 0; i < len(runs)-1; i++ {
		if unicode.IsUpper(runs[i][0]) && unicode.IsLower(runs[i+1][0]) {
			runs[i+1] = append([]rune{runs[i][len(runs[i])-1]}, runs[i+1]...)
			runs[i] = runs[i][:len(runs[i])-1]
		}
	}
	var words []string
	for _, run := range runs {
		if word := string(run); strings.TrimSpace(word) != "" && word != "_" {
			words = append(words, word)
		}
	}
	return words
}

func (t TextTable) markdownFieldName(index int) string {
	if index < len(t.FieldNames) && t.FieldNames[index] != "" {
		return t.FieldNames[index]
	}
	return t.Headers[index].String()
}

// escapeMarkdownTableCell keeps a cell on one pipe-table line: newlines
// become <br> and pipes are escaped.
func escapeMarkdownTableCell(s string) string {
	return escapeMarkdownPipes(markdownNewlinesToBr(s))
}

func markdownNewlinesToBr(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\n", "<br>")
}
