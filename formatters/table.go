package formatters

import (
	"bytes"

	"github.com/flanksource/clicky/api"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/samber/lo"
)

type Table struct {
	Headers     TextList
	Rows        []TextList
	Interactive bool
}

type TextList []api.Textable

func (t TextList) String() []string {
	result := make([]string, len(t))
	for i, text := range t {
		result[i] = text.String()
	}
	return result
}
func (t TextList) ANSI() []string {
	result := make([]string, len(t))
	for i, text := range t {
		result[i] = text.ANSI()
	}
	return result
}

func (t TextList) HTML() []string {
	result := make([]string, len(t))
	for i, text := range t {
		result[i] = text.HTML()
	}
	return result
}

func (t Table) HTML() string {
	return t.render(renderer.NewHTML())
}

func (t Table) String() string {
	return t.render(renderer.NewBlueprint())
}

func (t Table) ANSI() string {
	return t.render(renderer.NewColorized())
}

func (t *Table) render(renderer tw.Renderer) string {
	if len(t.Headers) == 0 {
		return ""
	}

	// Create buffer to capture table output
	var buf bytes.Buffer

	width := api.GetTerminalWidth()

	// Create tablewriter instance with word wrapping enabled
	// Set reasonable table max width to enable wrapping (this is distributed across columns)
	table := tablewriter.NewTable(&buf,
		tablewriter.WithRowAutoWrap(tw.WrapTruncate),
		tablewriter.WithHeaderAutoFormat(tw.On),
		tablewriter.WithMaxWidth(width),
		tablewriter.WithRenderer(renderer),
	)

	table.Header(lo.ToAnySlice(t.Headers.String())...)

	for _, row := range t.Rows {
		if err := table.Append(lo.ToAnySlice(row.ANSI())...); err != nil {
			return err.Error()
		}
	}

	// Render the table
	if err := table.Render(); err != nil {
		return err.Error()
	}

	return buf.String()

}
