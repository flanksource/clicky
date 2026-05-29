package api

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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

		Expect(func() { table.String() }).NotTo(Panic())
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
