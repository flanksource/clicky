package api

import (
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type sortableTableRow struct {
	ID         string    `json:"id"`
	Name       string    `json:"name" pretty:"label=Display Name" sort:"name"`
	UpdatedGMT time.Time `json:"updatedGMT" pretty:"label=Updated" sort:"updated"`
}

func (sortableTableRow) Columns() []ColumnDef {
	return []ColumnDef{
		Column("Name").Label("Display Name").Build(),
		Column("updatedGMT").Label("Updated").Build(),
		Column("id").Label("ID").Build(),
	}
}

func (r sortableTableRow) Row() map[string]any {
	return map[string]any{
		"Name":       r.Name,
		"updatedGMT": r.UpdatedGMT,
		"id":         r.ID,
	}
}

type unmatchedSortableTableRow struct {
	Name string `json:"name" sort:"name"`
}

func (unmatchedSortableTableRow) Columns() []ColumnDef {
	return []ColumnDef{Column("status").Build()}
}

func (unmatchedSortableTableRow) Row() map[string]any {
	return map[string]any{"status": "ready"}
}

var _ = Describe("Sortable columns", func() {
	It("reads public sort keys from TableProvider row tags for populated and empty tables", func() {
		updated := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)
		populated := NewTableFrom([]sortableTableRow{{ID: "id-1", Name: "Example", UpdatedGMT: updated}})
		empty := NewTableFrom([]sortableTableRow{})

		for _, table := range []TextTable{populated, empty} {
			Expect(table.Columns).To(HaveLen(3))
			Expect(table.Columns[0].SortKey).To(Equal("name"))
			Expect(table.Columns[1].SortKey).To(Equal("updated"))
			Expect(table.Columns[2].SortKey).To(BeEmpty())
		}
	})

	It("reads public sort keys from empty slices of pointer rows", func() {
		table := NewTableFrom([]*sortableTableRow{})

		Expect(table.Columns).To(HaveLen(3))
		Expect(table.Columns[0].SortKey).To(Equal("name"))
		Expect(table.Columns[1].SortKey).To(Equal("updated"))
	})

	It("lets an explicit column sort key override a matching struct tag", func() {
		columns, err := MergeSortableColumns(reflect.TypeFor[sortableTableRow](), []ColumnDef{
			Column("Name").Label("Display Name").SortKey("display-name").Build(),
			Column("updatedGMT").Label("Updated").Build(),
			Column("id").Build(),
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(columns[0].SortKey).To(Equal("display-name"))
		Expect(columns[1].SortKey).To(Equal("updated"))
	})

	It("fails when a declared sort field cannot be matched to a provider column", func() {
		_, err := MergeSortableColumns(
			reflect.TypeFor[unmatchedSortableTableRow](),
			unmatchedSortableTableRow{}.Columns(),
		)

		Expect(err).To(MatchError(ContainSubstring("sort field Name")))
	})

	It("reads sort keys for reflected struct columns", func() {
		fields, err := NewStructParser().getTableFields(reflect.ValueOf(sortableTableRow{}))

		Expect(err).NotTo(HaveOccurred())
		Expect(fields).To(ContainElements(
			And(HaveField("Name", "name"), HaveField("SortKey", "name")),
			And(HaveField("Name", "updatedGMT"), HaveField("SortKey", "updated")),
		))
	})
})
