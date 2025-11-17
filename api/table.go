package api

import (
	"bytes"

	"github.com/flanksource/commons/logger"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/samber/lo"
)

func (t TextTable) HTML() string {
	return t.render(renderer.NewHTML(), TransformerHTML)
}

func (t TextTable) String() string {
	return t.render(renderer.NewBlueprint(), TransformerString)
}

func (t TextTable) ANSI() string {
	return t.render(renderer.NewColorized(), TransformerANSI)
}

func (t TextTable) Markdown() string {
	return t.render(renderer.NewMarkdown(), TransformerMarkdown)
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
	logger.Infof("Rendering table with max width: %d", width)

	// Create tablewriter instance with word wrapping enabled
	// Set reasonable table max width to enable wrapping (this is distributed across columns)
	table := tablewriter.NewTable(&buf,
		tablewriter.WithRowAutoWrap(tw.WrapNone),
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
