//go:build !wasm

package api

import (
	"bytes"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

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

	headers := make([]any, len(t.Headers))
	for i, header := range t.Headers {
		headers[i] = markdownTableCell(Text{Content: header.String()}, t.markdownColumnWidth(i), options)
	}
	table.Header(headers...)

	for _, row := range t.Rows {
		values := make([]any, len(t.Headers))
		for i := range t.Headers {
			if cell, ok := row[t.markdownFieldName(i)]; ok {
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

func markdownTableCell(value Textable, width int, options MarkdownOptions) string {
	return escapeMarkdownTableCell(markdownCellText(value, width, options))
}
