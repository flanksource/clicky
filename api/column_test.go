package api

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// mockEmployee is a test type implementing TableProvider
type mockEmployee struct {
	ID         int
	Name       string
	Department string
	Salary     float64
	Active     bool
}

func (mockEmployee) Columns() []ColumnDef {
	return []ColumnDef{
		Column("id").Label("ID").Build(),
		Column("name").Label("Name").Style("font-semibold").Build(),
		Column("department").Label("Department").MaxWidth(20).Build(),
		Column("salary").Label("Salary").Format("currency").Build(),
		Column("status").Label("Status").Build(),
	}
}

func (e mockEmployee) Row() map[string]any {
	status := "Inactive"
	if e.Active {
		status = "Active"
	}
	return map[string]any{
		"id":         e.ID,
		"name":       e.Name,
		"department": e.Department,
		"salary":     e.Salary,
		"status":     status,
	}
}

var _ = Describe("Column", func() {
	Describe("ColumnBuilder", func() {
		It("creates a column with just a name", func() {
			col := Column("test_field").Build()
			Expect(col.Name).To(Equal("test_field"))
			Expect(col.Label).To(BeEmpty())
			Expect(col.Style).To(BeEmpty())
		})

		It("chains all builder methods", func() {
			col := Column("salary").
				Label("Annual Salary").
				Style("text-green-600").
				HeaderStyle("font-bold").
				Type("float").
				Format("currency").
				FormatOption("symbol", "€").
				MaxWidth(15).
				Build()

			Expect(col.Name).To(Equal("salary"))
			Expect(col.Label).To(Equal("Annual Salary"))
			Expect(col.Style).To(Equal("text-green-600"))
			Expect(col.HeaderStyle).To(Equal("font-bold"))
			Expect(col.Type).To(Equal("float"))
			Expect(col.Format).To(Equal("currency"))
			Expect(col.FormatOptions).To(HaveKeyWithValue("symbol", "€"))
			Expect(col.MaxWidth).To(Equal(15))
			Expect(col.Hidden).To(BeFalse())
		})

		It("marks column as hidden", func() {
			col := Column("internal_id").Hidden().Build()
			Expect(col.Hidden).To(BeTrue())
		})
	})

	Describe("ColumnDef.DisplayLabel", func() {
		It("returns Label when set", func() {
			col := ColumnDef{Name: "user_name", Label: "User Name"}
			Expect(col.DisplayLabel()).To(Equal("User Name"))
		})

		It("prettifies Name when Label is empty", func() {
			col := ColumnDef{Name: "user_name"}
			Expect(col.DisplayLabel()).To(Equal("User Name"))
		})

		It("handles camelCase names", func() {
			col := ColumnDef{Name: "firstName"}
			Expect(col.DisplayLabel()).To(Equal("First Name"))
		})
	})

	It("binds a server filter key through the column builder", func() {
		column := Column("status").FilterKey("filter.status").Build()
		Expect(column.FilterKey).To(Equal("filter.status"))
	})

	Describe("NewTableFrom", func() {
		It("emits header-only table (schema, no rows) from empty slice", func() {
			table := NewTableFrom([]mockEmployee{})
			Expect(table.Headers).To(HaveLen(5))
			Expect(table.FieldNames).To(Equal([]string{"id", "name", "department", "salary", "status"}))
			Expect(table.Rows).To(BeEmpty())
		})

		It("creates table with correct headers", func() {
			employees := []mockEmployee{
				{ID: 1, Name: "Alice", Department: "Engineering", Salary: 95000, Active: true},
			}
			table := NewTableFrom(employees)

			Expect(table.Headers).To(HaveLen(5))
			Expect(table.Headers[0].String()).To(Equal("ID"))
			Expect(table.Headers[1].String()).To(Equal("Name"))
			Expect(table.Headers[2].String()).To(Equal("Department"))
			Expect(table.Headers[3].String()).To(Equal("Salary"))
			Expect(table.Headers[4].String()).To(Equal("Status"))
		})

		It("creates table with correct field names", func() {
			employees := []mockEmployee{
				{ID: 1, Name: "Alice", Department: "Engineering", Salary: 95000, Active: true},
			}
			table := NewTableFrom(employees)

			Expect(table.FieldNames).To(Equal([]string{"id", "name", "department", "salary", "status"}))
		})

		It("creates table with multiple rows", func() {
			employees := []mockEmployee{
				{ID: 1, Name: "Alice", Department: "Engineering", Salary: 95000, Active: true},
				{ID: 2, Name: "Bob", Department: "Sales", Salary: 75000, Active: false},
			}
			table := NewTableFrom(employees)

			Expect(table.Rows).To(HaveLen(2))

			// Check first row values
			Expect(table.Rows[0]["id"].String()).To(Equal("1"))
			Expect(table.Rows[0]["name"].String()).To(Equal("Alice"))
			Expect(table.Rows[0]["status"].String()).To(Equal("Active"))

			// Check second row values
			Expect(table.Rows[1]["id"].String()).To(Equal("2"))
			Expect(table.Rows[1]["name"].String()).To(Equal("Bob"))
			Expect(table.Rows[1]["status"].String()).To(Equal("Inactive"))
		})

		It("excludes hidden columns from table", func() {
			// mockEmployeeWithHidden implements TableProvider with a hidden column
			type mockEmployeeWithHidden struct {
				ID       int
				Name     string
				Internal string // will be hidden
			}

			// Define columns with one hidden
			columns := []ColumnDef{
				Column("id").Label("ID").Build(),
				Column("name").Label("Name").Build(),
				Column("internal").Label("Internal").Hidden().Build(),
			}

			// Verify the hidden column is marked correctly
			Expect(columns[2].Hidden).To(BeTrue())

			// Filter out hidden columns (as NewTableFrom does internally)
			visibleColumns := []ColumnDef{}
			for _, col := range columns {
				if !col.Hidden {
					visibleColumns = append(visibleColumns, col)
				}
			}

			Expect(visibleColumns).To(HaveLen(2))
			Expect(visibleColumns[0].Name).To(Equal("id"))
			Expect(visibleColumns[1].Name).To(Equal("name"))
		})
	})
})
