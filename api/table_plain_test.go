//go:build !wasm

package api

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func plainTestTable(headers []string, rows ...[]any) TextTable {
	table := TextTable{}
	for _, header := range headers {
		table.Headers = append(table.Headers, Text{Content: header})
	}
	for _, cells := range rows {
		row := TableRow{}
		for i, cell := range cells {
			row[headers[i]] = NewTypedValue(cell)
		}
		table.Rows = append(table.Rows, row)
	}
	return table
}

func plainLines(lines ...string) string {
	return strings.Join(lines, "\n")
}

var _ = Describe("plainTable", func() {
	DescribeTable("lays out aligned columns under a header rule",
		func(table TextTable, want string) {
			Expect(table.plainTable(false)).To(Equal(want))
		},
		Entry("no headers renders nothing", TextTable{}, ""),
		Entry("headers only renders the header and its rule",
			plainTestTable([]string{"Name", "Count"}),
			plainLines(
				"Name  Count",
				"────  ─────",
			)),
		Entry("columns that are empty in every row are dropped",
			plainTestTable([]string{"Name", "Unused", "Note"},
				[]any{"alpha", "", "one"},
				[]any{"beta", "", "two"}),
			plainLines(
				"Name   Note",
				"─────  ────",
				"alpha  one",
				"beta   two",
			)),
		Entry("wide and combining characters pad by display width",
			plainTestTable([]string{"Name", "Note"},
				[]any{"日本", "wide"},
				[]any{"βeta", "greek"},
				[]any{"é", "combining"}),
			plainLines(
				"Name  Note",
				"────  ─────────",
				"日本  wide",
				"βeta  greek",
				"é     combining",
			)),
		Entry("multi-line cells expand into physical rows under their column",
			plainTestTable([]string{"Name", "Note", "Tail"},
				[]any{"alpha", "pipe | inside", "x"},
				[]any{"βeta", "multi\r\nline", "y\nz\nw"}),
			plainLines(
				"Name   Note           Tail",
				"─────  ─────────────  ────",
				"alpha  pipe | inside  x",
				"βeta   multi          y",
				"       line           z",
				"                      w",
			)),
		Entry("a trailing empty cell leaves no trailing whitespace",
			plainTestTable([]string{"Name", "Note"},
				[]any{"alpha", "one"},
				[]any{"gamma", ""}),
			plainLines(
				"Name   Note",
				"─────  ────",
				"alpha  one",
				"gamma",
			)),
	)

	It("pads ANSI cells by their visible width, not their byte length", func() {
		table := plainTestTable([]string{"Name", "Note"},
			[]any{Text{Content: "alpha", Style: "text-red-600 font-bold"}, "one"},
			[]any{"b", Text{Content: "two", Style: "text-green-500"}})

		rendered := table.plainTable(true)

		Expect(rendered).To(ContainSubstring("\x1b["))
		Expect(ansi.Strip(rendered)).To(Equal(plainLines(
			"Name   Note",
			"─────  ────",
			"alpha  one",
			"b      two",
		)))
	})
})
