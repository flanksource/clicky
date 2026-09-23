//go:build !wasm

package api

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type parityRow struct {
	Name   string
	Count  int64
	Amount float64
	Note   string
}

func (r parityRow) Columns() []ColumnDef {
	return []ColumnDef{
		Column("display_name").Label("display_name").Build(),
		Column("count").Label("Count").Build(),
		Column("amount").Label("Amount").Build(),
		Column("note").Label("Note").Build(),
		Column("unused").Label("Unused").Build(),
	}
}

func (r parityRow) Row() map[string]any {
	return map[string]any{"display_name": r.Name, "count": r.Count, "amount": r.Amount, "note": r.Note, "unused": ""}
}

// markdownTableCells splits each pipe-table line into trimmed cells, ignoring
// escaped pipes and the alignment row, so padding/alignment differences
// between the native and plain renderers do not matter.
func markdownTableCells(rendered string) [][]string {
	var table [][]string
	for _, line := range markdownTableLines(rendered) {
		inner := strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
		cells := strings.Split(strings.ReplaceAll(inner, `\|`, "\x00"), "|")
		for i, cell := range cells {
			cells[i] = strings.ReplaceAll(strings.TrimSpace(cell), "\x00", `\|`)
		}
		if strings.Trim(strings.Join(cells, ""), ":-") == "" {
			continue
		}
		table = append(table, cells)
	}
	return table
}

var _ = Describe("plain markdown table parity with tablewriter", func() {
	DescribeTable("renders the same cells as the native tablewriter table",
		func(table TextTable) {
			native := table.MarkdownWithOptions(MarkdownOptions{})
			plain := table.plainMarkdown(MarkdownOptions{})

			Expect(markdownTableCells(plain)).To(Equal(markdownTableCells(native)))
		},
		Entry("pipes, newlines, unicode, big numbers and an all-empty column",
			NewTableFrom([]parityRow{
				{Name: "alpha", Count: 1234567, Amount: 12.5, Note: "pipe | inside"},
				{Name: "βeta", Count: -42, Amount: 0.001, Note: "multi\nline"},
				{Name: "gamma", Count: 9223372036854775807, Amount: 1e7},
			})),
		Entry("headers without rows",
			TextTable{Headers: TextList{Text{Content: "name"}, Text{Content: "note"}}}),
		Entry("header auto-format edge cases",
			TextTable{Headers: TextList{
				Text{Content: "field_name"}, Text{Content: "meta.name"}, Text{Content: "1.5"},
				Text{Content: ".hidden"}, Text{Content: "v1."}, Text{Content: "   "}, Text{Content: "βeta"},
				Text{Content: "HTTPServer"}, Text{Content: "a|b"}, Text{Content: "two\nlines"}, Text{Content: "__"},
			}}),
		Entry("styled and width-limited cells",
			NewTableFrom([]markdownTableRow{
				{value: Text{}.Append("abcdefghijklmno", "text-green-600"), width: 10},
				{value: strings.Repeat("b", 205)},
			})),
	)
})
