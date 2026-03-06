// ABOUTME: Tests for collapsible row support in HTML table renderers.
// ABOUTME: Validates that rows with detail content render expandable sections.
package api

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// detailEmployee implements TableProvider and DetailProvider
type detailEmployee struct {
	ID     int
	Name   string
	Detail Textable
}

func (detailEmployee) Columns() []ColumnDef {
	return []ColumnDef{
		Column("id").Label("ID").Build(),
		Column("name").Label("Name").Build(),
	}
}

func (e detailEmployee) Row() map[string]any {
	return map[string]any{"id": e.ID, "name": e.Name}
}

func (e detailEmployee) RowDetail() Textable {
	return e.Detail
}

var _ = Describe("Collapsible Rows", func() {
	Describe("DetailProvider interface", func() {
		It("is detected by NewTableFrom and populates RowDetail", func() {
			items := []detailEmployee{
				{ID: 1, Name: "Alice", Detail: Text{Content: "Alice's full profile"}},
				{ID: 2, Name: "Bob", Detail: nil},
				{ID: 3, Name: "Charlie", Detail: Text{Content: "Charlie's notes"}},
			}
			table := NewTableFrom(items)

			Expect(table.RowDetail).To(HaveLen(3))
			Expect(table.RowDetail[0]).NotTo(BeNil())
			Expect(table.RowDetail[0].String()).To(Equal("Alice's full profile"))
			Expect(table.RowDetail[1]).To(BeNil())
			Expect(table.RowDetail[2]).NotTo(BeNil())
			Expect(table.RowDetail[2].String()).To(Equal("Charlie's notes"))
		})

		It("leaves RowDetail nil when items don't implement DetailProvider", func() {
			items := []mockEmployee{
				{ID: 1, Name: "Alice", Department: "Eng", Salary: 100000, Active: true},
			}
			table := NewTableFrom(items)
			Expect(table.RowDetail).To(BeNil())
		})
	})

	Describe("WithoutEmptyColumns preserves RowDetail", func() {
		It("carries RowDetail through filtering", func() {
			detail := Text{Content: "detail content"}
			table := TextTable{
				Headers:    TextList{Text{Content: "Name"}, Text{Content: "Empty"}},
				FieldNames: []string{"name", "empty"},
				Rows: []TableRow{
					{"name": TypedValue{Textable: Text{Content: "Alice"}}, "empty": TypedValue{Textable: Text{}}},
				},
				RowDetail: []Textable{detail},
			}

			filtered := table.WithoutEmptyColumns()
			Expect(filtered.RowDetail).To(HaveLen(1))
			Expect(filtered.RowDetail[0].String()).To(Equal("detail content"))
		})
	})

	Describe("HTML rendering with collapsible rows", func() {
		var table TextTable

		BeforeEach(func() {
			table = TextTable{
				Headers:    TextList{Text{Content: "Name"}, Text{Content: "Age"}},
				FieldNames: []string{"name", "age"},
				Columns: []PrettyField{
					{Name: "name", Label: "Name"},
					{Name: "age", Label: "Age"},
				},
				Rows: []TableRow{
					{"name": TypedValue{Textable: Text{Content: "Alice"}}, "age": TypedValue{Textable: Text{Content: "30"}}},
					{"name": TypedValue{Textable: Text{Content: "Bob"}}, "age": TypedValue{Textable: Text{Content: "25"}}},
				},
				RowDetail: []Textable{
					Text{Content: "Alice detail info"},
					nil, // Bob has no detail
				},
			}
		})

		It("renders expandable rows in interactive HTML", func() {
			html := table.HTML()

			// Row with detail should have a click handler
			Expect(html).To(ContainSubstring("Alice detail info"))
			// Row without detail should not have expand UI
			Expect(html).NotTo(ContainSubstring("Bob detail info"))
		})

		It("renders expandable rows in static HTML", func() {
			html := table.StaticHTML()

			// Detail row should be present but hidden by default
			Expect(html).To(ContainSubstring("Alice detail info"))
			Expect(html).NotTo(ContainSubstring("Bob detail info"))
		})

		It("renders expandable rows in compact HTML", func() {
			html := table.CompactHTML()

			Expect(html).To(ContainSubstring("Alice detail info"))
			Expect(html).NotTo(ContainSubstring("Bob detail info"))
		})

		It("does not render detail rows when RowDetail is nil", func() {
			table.RowDetail = nil
			html := table.StaticHTML()

			// Should be a normal table without any expand/collapse UI
			Expect(html).NotTo(ContainSubstring("x-data"))
			Expect(html).NotTo(ContainSubstring("detail"))
		})

		It("uses Alpine.js for expand/collapse in static HTML", func() {
			html := table.StaticHTML()

			// Should use Alpine.js x-data for state management
			Expect(html).To(ContainSubstring("x-data"))
			Expect(html).To(ContainSubstring("x-show"))
		})

		It("spans detail row across all columns", func() {
			html := table.StaticHTML()

			// Detail row should have colspan spanning all columns (2 data + 1 chevron)
			Expect(html).To(ContainSubstring(`colspan="3"`))
		})

		It("includes chevron indicator on expandable rows", func() {
			html := table.StaticHTML()

			// Should have a chevron icon for expand state
			Expect(strings.Contains(html, "chevron") || strings.Contains(html, "▶")).To(BeTrue())
		})
	})
})
