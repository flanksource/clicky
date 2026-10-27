package api

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("plainMarkdownTable", func() {
	DescribeTable("renders a GFM pipe table without padding",
		func(headers []string, rows [][]string, want string) {
			Expect(plainMarkdownTable(headers, rows)).To(Equal(want))
		},
		Entry("no headers renders nothing", nil, [][]string{{"orphan"}}, ""),
		Entry("headers without rows keep the separator",
			[]string{"NAME", "COUNT"}, nil,
			"| NAME | COUNT |\n| --- | --- |\n"),
		Entry("one row per record, empty cells kept",
			[]string{"NAME", "NOTE"}, [][]string{{"alpha", ""}, {"beta", "x"}},
			"| NAME | NOTE |\n| --- | --- |\n| alpha |  |\n| beta | x |\n"),
		Entry("pipes are escaped in headers and cells",
			[]string{"A|B"}, [][]string{{"pipe | inside"}},
			"| A\\|B |\n| --- |\n| pipe \\| inside |\n"),
		Entry("LF, CRLF and CR become <br>",
			[]string{"NOTE"}, [][]string{{"first\nsecond\r\n\rthird"}},
			"| NOTE |\n| --- |\n| first<br>second<br><br>third |\n"),
		Entry("unicode passes through without width padding",
			[]string{"名前"}, [][]string{{"βeta"}, {"日本語テキスト"}},
			"| 名前 |\n| --- |\n| βeta |\n| 日本語テキスト |\n"),
	)
})

var _ = Describe("markdownHeaderTitle", func() {
	DescribeTable("formats headers like tablewriter's header auto-format",
		func(header, want string) {
			Expect(markdownHeaderTitle(header)).To(Equal(want))
		},
		Entry("uppercases", "name", "NAME"),
		Entry("underscores become spaces", "field_name", "FIELD NAME"),
		Entry("camelCase splits into words", "displayName", "DISPLAY NAME"),
		Entry("acronym hands its last letter to the next word", "HTTPServer", "HTTP SERVER"),
		Entry("digits split from letters", "v1", "V 1"),
		Entry("punctuation becomes its own word", "meta.name", "META . NAME"),
		Entry("surrounding whitespace is trimmed", "  count ", "COUNT"),
		Entry("whitespace-only is dropped", "   ", ""),
		Entry("underscore runs collapse to one space", "__", " "),
		Entry("empty stays empty", "", ""),
		Entry("unicode uppercases", "βeta", "ΒETA"),
	)
})

var _ = Describe("TextTable.plainMarkdown", func() {
	It("selects cells by field name, formats headers and keeps all-empty columns", func() {
		table := TextTable{
			Headers:    TextList{Text{Content: "display_name"}, Text{Content: "Empty"}, Text{Content: "note"}},
			FieldNames: []string{"name", "", "note"},
			Rows: []TableRow{
				{"name": NewTypedValue("alpha"), "note": NewTypedValue("a|b")},
				{"name": NewTypedValue("beta"), "Empty": NewTypedValue("  ")},
			},
		}

		Expect(table.plainMarkdown(MarkdownOptions{})).To(Equal(
			"\n| DISPLAY NAME | EMPTY | NOTE |\n| --- | --- | --- |\n| alpha |  | a\\|b |\n| beta |  |  |\n"))
	})

	It("links each row's detail to a note under the table, numbered in row order", func() {
		table := TextTable{
			Headers:    TextList{Text{Content: "status"}, Text{Content: "name"}},
			FieldNames: []string{"status", "name"},
			Rows: []TableRow{
				{"status": NewTypedValue("Changed"), "name": NewTypedValue("Lapse Policy")},
				{"status": NewTypedValue("Added"), "name": NewTypedValue("Renew")},
				{"status": NewTypedValue("Removed"), "name": NewTypedValue("Enroll")},
			},
			RowDetail: []Textable{Text{Content: "lapse diff"}, nil, Text{Content: "enroll diff"}},
		}

		Expect(table.plainMarkdown(MarkdownOptions{})).To(Equal("\n" +
			"| STATUS | NAME |\n| --- | --- |\n" +
			"| Changed [¹](#detail-1-changed-lapse-policy) | Lapse Policy |\n" +
			"| Added | Renew |\n" +
			"| Removed [²](#detail-2-removed-enroll) | Enroll |\n" +
			"\n#### ¹ Changed · Lapse Policy <a id=\"detail-1-changed-lapse-policy\"></a>\n\nlapse diff\n" +
			"\n#### ² Removed · Enroll <a id=\"detail-2-removed-enroll\"></a>\n\nenroll diff\n"))
	})

	It("renders nothing without headers", func() {
		Expect(TextTable{}.plainMarkdown(MarkdownOptions{})).To(BeEmpty())
	})
})
