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

func plainTestNode(label string, children ...TextTree) TextTree {
	return TextTree{Node: Text{Content: label}, Children: children}
}

var plainTreeFixtures = map[string]TextTree{
	"single node": plainTestNode("root"),
	"nested children": plainTestNode("root",
		plainTestNode("child-a", plainTestNode("leaf")),
		plainTestNode("child-b")),
	"rootless with several children": {Children: []TextTree{
		plainTestNode("one", plainTestNode("one-a")),
		plainTestNode("two"),
	}},
	"rootless with one child collapses into the child": {Children: []TextTree{
		plainTestNode("only", plainTestNode("leaf")),
	}},
	"multi-line labels with blank lines": plainTestNode("title\n\nsubtitle",
		plainTestNode("first\nsecond", plainTestNode("deep\n  indented")),
		plainTestNode("last\nline")),
	"tabbed labels": plainTestNode("key\tvalue",
		plainTestNode("a\tb\nc\td")),
	"unicode labels": plainTestNode("日本",
		plainTestNode("βeta", plainTestNode("é"))),
	"nodeless child keeps its connector": plainTestNode("root",
		TextTree{Children: []TextTree{plainTestNode("a"), plainTestNode("b")}},
		plainTestNode("after")),
	"styled labels": {
		Node: Text{Content: "root", Style: "text-blue-600 font-bold"},
		Children: []TextTree{
			{Node: Text{Content: "red\nwrapped", Style: "text-red-600"}},
			{Node: Text{Content: "green", Style: "text-green-500"}},
		},
	},
}

var _ = Describe("plainTree", func() {
	It("renders nothing for an empty tree", func() {
		Expect(TextTree{}.plainTree(false)).To(BeEmpty())
		Expect(TextTree{}.plainTree(true)).To(BeEmpty())
	})

	It("draws connectors with continuation gutters under multi-line labels", func() {
		tree := plainTestNode("root",
			plainTestNode("first\nsecond", plainTestNode("leaf")),
			plainTestNode("last\nline"))

		Expect(tree.plainTree(false)).To(Equal(plainLines(
			"root",
			"├── first",
			"│   second",
			"│   ╰── leaf",
			"╰── last",
			"    line",
		)))
	})

	It("keeps each label's ANSI styling", func() {
		Expect(plainTreeFixtures["styled labels"].plainTree(true)).To(ContainSubstring("\x1b["))
	})

	for name, tree := range plainTreeFixtures {
		It("matches the native plain render for "+name, func() {
			Expect(tree.plainTree(false)).To(Equal(tree.String()))
		})
		It("matches the native ANSI render for "+name, func() {
			Expect(tree.plainTree(true)).To(Equal(tree.ANSI()))
		})
	}
})
