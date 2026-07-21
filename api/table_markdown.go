package api

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/flanksource/clicky/api/tailwind"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/samber/lo"
)

const defaultMarkdownColumnWidth = 200

func (t TextTable) MarkdownWithOptions(options MarkdownOptions) string {
	if len(t.Headers) == 0 {
		return ""
	}

	var buf bytes.Buffer
	table := tablewriter.NewTable(&buf,
		tablewriter.WithRowAutoWrap(tw.WrapNone),
		tablewriter.WithHeaderAutoFormat(tw.On),
		tablewriter.WithBehavior(tw.Behavior{AutoHide: tw.On}),
		tablewriter.WithRenderer(renderer.NewMarkdown()),
	)

	headers := make([]string, len(t.Headers))
	for i, header := range t.Headers {
		headers[i] = markdownTableCell(Text{Content: header.String()}, t.markdownColumnWidth(i), options)
	}
	table.Header(lo.ToAnySlice(headers)...)

	for _, row := range t.Rows {
		values := make([]any, len(t.Headers))
		for i, header := range t.Headers {
			fieldName := header.String()
			if i < len(t.FieldNames) && t.FieldNames[i] != "" {
				fieldName = t.FieldNames[i]
			}
			if cell, ok := row[fieldName]; ok {
				values[i] = markdownTableCell(cell, t.markdownColumnWidth(i), options)
			}
		}
		if err := table.Append(values...); err != nil {
			return err.Error()
		}
	}

	if err := table.Render(); err != nil {
		return err.Error()
	}
	return "\n" + buf.String()
}

func (t TextTable) markdownColumnWidth(index int) int {
	if index < len(t.Columns) {
		if width := tailwind.ParseStyle(t.Columns[index].Style).MaxWidth; width > 0 {
			return width
		}
	}
	return defaultMarkdownColumnWidth
}

func markdownTableCell(value Textable, width int, options MarkdownOptions) string {
	limited := Text{Style: fmt.Sprintf("max-w-[%dch] truncate-suffix", width)}.Add(value)
	markdown := RenderMarkdown(limited, options)
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	markdown = strings.ReplaceAll(markdown, "\r", "\n")
	markdown = strings.ReplaceAll(markdown, "\n", "<br>")
	return escapeMarkdownPipes(markdown)
}
