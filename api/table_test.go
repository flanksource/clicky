package api

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type cappedWidthTableRow struct{}

func (cappedWidthTableRow) Columns() []ColumnDef {
	return []ColumnDef{
		Column("agent").Label("Agent").MaxWidth(12).Build(),
		Column("session").Label("Session").MaxWidth(8).Build(),
		Column("title").Label("Title").Build(),
	}
}

type shrinkingCappedWidthTableRow struct{}

func (shrinkingCappedWidthTableRow) Columns() []ColumnDef {
	return []ColumnDef{
		Column("prompt").Label("Prompt").MaxWidth(30).Build(),
		Column("usage").Label("Usage").Build(),
	}
}

func (shrinkingCappedWidthTableRow) Row() map[string]any {
	return map[string]any{
		"prompt": "A capped column must yield space when the terminal is narrow",
		"usage":  "$1.25",
	}
}

func (cappedWidthTableRow) Row() map[string]any {
	return map[string]any{
		"agent":   "codex-agent",
		"session": "019f5c3c",
		"title":   "Title uses all terminal space left after capped columns",
	}
}

var _ = Describe("WithoutEmptyColumns", func() {
	It("removes columns where every row is empty", func() {
		table := TextTable{
			Headers:    TextList{Text{Content: "Name"}, Text{Content: "Age"}, Text{Content: "Notes"}},
			FieldNames: []string{"name", "age", "notes"},
		}
		table.Rows = []TableRow{
			{"name": TypedValue{Textable: Text{Content: "Alice"}}, "age": TypedValue{Textable: Text{Content: "30"}}, "notes": TypedValue{Textable: Text{}}},
			{"name": TypedValue{Textable: Text{Content: "Bob"}}, "age": TypedValue{Textable: Text{Content: "25"}}, "notes": TypedValue{Textable: Text{}}},
		}

		filtered := table.WithoutEmptyColumns()
		Expect(filtered.Headers).To(HaveLen(2))
		Expect(filtered.Headers[0].String()).To(Equal("Name"))
		Expect(filtered.Headers[1].String()).To(Equal("Age"))
		Expect(filtered.FieldNames).To(Equal([]string{"name", "age"}))
		Expect(filtered.Rows).To(HaveLen(2))
		Expect(filtered.Rows[0]).To(HaveKey("name"))
		Expect(filtered.Rows[0]).To(HaveKey("age"))
		Expect(filtered.Rows[0]).NotTo(HaveKey("notes"))
	})

	It("keeps all columns when none are fully empty", func() {
		table := TextTable{
			Headers:    TextList{Text{Content: "A"}, Text{Content: "B"}},
			FieldNames: []string{"a", "b"},
		}
		table.Rows = []TableRow{
			{"a": TypedValue{Textable: Text{Content: "1"}}, "b": TypedValue{Textable: Text{Content: "2"}}},
		}

		filtered := table.WithoutEmptyColumns()
		Expect(filtered.Headers).To(HaveLen(2))
	})

	It("keeps column if at least one row has a value", func() {
		table := TextTable{
			Headers:    TextList{Text{Content: "X"}, Text{Content: "Y"}},
			FieldNames: []string{"x", "y"},
		}
		table.Rows = []TableRow{
			{"x": TypedValue{Textable: Text{Content: "val"}}, "y": TypedValue{Textable: Text{}}},
			{"x": TypedValue{Textable: Text{}}, "y": TypedValue{Textable: Text{Content: "val"}}},
		}

		filtered := table.WithoutEmptyColumns()
		Expect(filtered.Headers).To(HaveLen(2))
	})

	It("returns same table when no rows exist", func() {
		table := TextTable{
			Headers:    TextList{Text{Content: "A"}},
			FieldNames: []string{"a"},
		}
		filtered := table.WithoutEmptyColumns()
		Expect(filtered.Headers).To(HaveLen(1))
	})

	It("treats whitespace-only values as empty", func() {
		table := TextTable{
			Headers:    TextList{Text{Content: "Col"}},
			FieldNames: []string{"col"},
		}
		table.Rows = []TableRow{
			{"col": TypedValue{Textable: Text{Content: "  "}}},
		}

		filtered := table.WithoutEmptyColumns()
		Expect(filtered.Headers).To(BeEmpty())
	})

	It("does not panic when all columns are filtered out during render", func() {
		table := TextTable{
			Headers:    TextList{Text{Content: "Col"}},
			FieldNames: []string{"col"},
		}
		table.Rows = []TableRow{
			{"col": TypedValue{Textable: Text{Content: "  "}}},
		}

		Expect(func() { _ = table.String() }).NotTo(Panic())
		Expect(table.String()).To(BeEmpty())
		Expect(func() { table.ANSI() }).NotTo(Panic())
	})
})

var _ = Describe("TextTable Markdown pipe escaping", func() {
	It("escapes a literal pipe in a cell so the GFM table stays intact", func() {
		t := TextTable{
			Headers:    TextList{Text{Content: "A"}, Text{Content: "B"}},
			FieldNames: []string{"a", "b"},
			Rows: []TableRow{
				{"a": TypedValue{Textable: Text{Content: "x|y"}}, "b": TypedValue{Textable: Text{Content: "ok"}}},
			},
		}
		Expect(t.Markdown()).To(ContainSubstring(`x\|y`))
		Expect(t.Markdown()).NotTo(ContainSubstring(`| x|y |`))
	})
})

var _ = Describe("TextTable terminal column widths", func() {
	It("keeps capped columns compact and gives uncapped columns the remaining width", func() {
		previousWidth := terminalWidth.Swap(120)
		DeferCleanup(func() { terminalWidth.Store(previousWidth) })

		rendered := NewTableFrom([]cappedWidthTableRow{{}}).String()
		var header string
		for _, line := range strings.Split(rendered, "\n") {
			if strings.Contains(line, "│Agent") {
				header = line
				break
			}
		}
		Expect(header).NotTo(BeEmpty())
		cells := strings.Split(strings.Trim(header, "│"), "│")
		Expect(cells).To(HaveLen(3))
		Expect(len([]rune(cells[0]))).To(BeNumerically("<=", 12))
		Expect(len([]rune(cells[1]))).To(BeNumerically("<=", 8))
		Expect(len([]rune(cells[2]))).To(BeNumerically(">", 80))
	})

	It("allows capped columns to shrink when content exceeds the terminal width", func() {
		previousWidth := terminalWidth.Swap(30)
		DeferCleanup(func() { terminalWidth.Store(previousWidth) })

		rendered := NewTableFrom([]shrinkingCappedWidthTableRow{{}}).String()
		Expect(rendered).To(ContainSubstring("$1.25"))
	})
})
