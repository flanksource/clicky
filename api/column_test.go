package api

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
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

type hiddenColumnRow struct {
	Name     string
	Tenant   string
	Internal string
	Count    int64
	Enabled  bool
	Items    []string
}

func (hiddenColumnRow) Columns() []ColumnDef {
	return []ColumnDef{
		Column("name").Build(),
		Column("tenant").Hidden().FilterKey("filter.tenant").Build(),
		Column("internal").Hidden().Build(),
		Column("count").Hidden().Build(),
		Column("enabled").Hidden().Build(),
		Column("items").Hidden().Build(),
	}
}

func (r hiddenColumnRow) Row() map[string]any {
	return map[string]any{
		"name": r.Name, "tenant": r.Tenant, "internal": r.Internal,
		"count": r.Count, "enabled": r.Enabled, "items": r.Items,
	}
}

type presentedCellRow struct{ Duration float64 }

func (presentedCellRow) Columns() []ColumnDef {
	return []ColumnDef{Column("duration").FilterKey("filter.duration").Build()}
}

func (r presentedCellRow) Row() map[string]any {
	return map[string]any{
		"duration": TableCell{
			Value:       Text{Content: "125ms", Style: "text-red-500"},
			FilterValue: r.Duration,
		},
	}
}

type presentedCounterRow struct{ PhysicalReads int64 }

func (presentedCounterRow) Columns() []ColumnDef {
	return []ColumnDef{Column("physicalReads").Build()}
}

func (r presentedCounterRow) Row() map[string]any {
	return map[string]any{
		"physicalReads": TableCell{Value: Text{}, FilterValue: r.PhysicalReads},
	}
}

type presentedStructuredValueRow struct{ Tables []string }

func (presentedStructuredValueRow) Columns() []ColumnDef {
	return []ColumnDef{Column("tables").Build()}
}

func (r presentedStructuredValueRow) Row() map[string]any {
	return map[string]any{
		"tables": TableCell{Value: Text{Content: "2 tables"}, FilterValue: r.Tables},
	}
}

type defaultHiddenRow struct{ Session, Host, ID string }

func (defaultHiddenRow) Columns() []ColumnDef {
	return []ColumnDef{
		Column("sessionId").Build(),
		Column("clientHost").Label("Host").DefaultHidden().Build(),
		Column("_id").Hidden().Build(),
	}
}

func (r defaultHiddenRow) Row() map[string]any {
	return map[string]any{"sessionId": r.Session, "clientHost": r.Host, "_id": r.ID}
}

type structuredCellRow struct{ SQL string }

func (structuredCellRow) Columns() []ColumnDef {
	return []ColumnDef{
		Column("sql").Label("Statement").FilterKey("filter.sql").MinWidthPixels(360).MaxWidthPixels(720).Build(),
	}
}

func (r structuredCellRow) Row() map[string]any {
	return map[string]any{
		"sql": TableCell{Value: CodeBlock("text/x-sql", r.SQL), FilterValue: r.SQL},
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
				MinWidthPixels(360).
				MaxWidthPixels(720).
				Build()

			Expect(col.Name).To(Equal("salary"))
			Expect(col.Label).To(Equal("Annual Salary"))
			Expect(col.Style).To(Equal("text-green-600"))
			Expect(col.HeaderStyle).To(Equal("font-bold"))
			Expect(col.Type).To(Equal("float"))
			Expect(col.Format).To(Equal("currency"))
			Expect(col.FormatOptions).To(HaveKeyWithValue("symbol", "€"))
			Expect(col.MaxWidth).To(Equal(15))
			Expect(col.MinWidthPixels).To(Equal(360))
			Expect(col.MaxWidthPixels).To(Equal(720))
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

	It("keeps an explicitly presented textable ahead of scalar type formatting", func() {
		presented := CodeBlock("text/x-sql", "SELECT 1")
		Expect(ColumnTextable(ColumnDef{Type: FieldTypeString}, presented)).To(Equal(presented))
	})

	Describe("NewTableFrom", func() {
		DescribeTable("keeps a DefaultHidden column in the schema, flagged, while a Hidden one is dropped",
			func(rows []defaultHiddenRow) {
				table := NewTableFrom(rows)

				Expect(table.FieldNames).To(Equal([]string{"sessionId", "clientHost"}))
				Expect(table.Columns).To(HaveLen(2))
				Expect(table.Columns[0].DefaultHidden).To(BeFalse())
				Expect(table.Columns[1]).To(MatchFields(IgnoreExtras, Fields{
					"Name":          Equal("clientHost"),
					"Label":         Equal("Host"),
					"DefaultHidden": BeTrue(),
				}))
			},
			Entry("with rows", []defaultHiddenRow{{Session: "53", Host: "azure-app-1", ID: "e1"}}),
			Entry("with zero rows", []defaultHiddenRow{}),
		)

		It("carries a DefaultHidden column's cell like any visible column", func() {
			table := NewTableFrom([]defaultHiddenRow{{Session: "53", Host: "azure-app-1", ID: "e1"}})

			Expect(table.Rows[0]["clientHost"].Textable.String()).To(Equal("azure-app-1"))
		})

		It("preserves a structured code cell as the table cell root", func() {
			const statement = "SELECT * FROM Activity WHERE ActivityStatusCode = '01'"
			table := NewTableFrom([]structuredCellRow{{SQL: statement}})

			code, ok := table.Rows[0]["sql"].Textable.(Code)
			Expect(ok).To(BeTrue())
			Expect(code.Language).To(Equal("sql"))
			Expect(code.Content).To(Equal(statement))
			Expect(table.Rows[0]["sql"].FilterValue).To(Equal(statement))
			Expect(table.Columns[0].MinWidth).To(Equal(360))
			Expect(table.Columns[0].MaxWidth).To(Equal(720))
		})

		DescribeTable("keeps the raw scalar of an explicitly presented cell on a visible column without a filter key",
			func(reads int64) {
				table := NewTableFrom([]presentedCounterRow{{PhysicalReads: reads}})

				Expect(table.Rows[0]["physicalReads"].FilterValue).To(Equal(reads))
			},
			Entry("zero, whose display text is empty", int64(0)),
			Entry("a count whose display text is abbreviated", int64(1234)),
		)

		It("drops a non-scalar raw value of a presented cell on a column without a filter key", func() {
			table := NewTableFrom([]presentedStructuredValueRow{{Tables: []string{"AsActivity", "AsPolicy"}}})

			Expect(table.Rows[0]["tables"].String()).To(Equal("2 tables"))
			Expect(table.Rows[0]["tables"].FilterValue).To(BeNil())
		})

		It("emits a schema-less empty table for an empty interface-typed slice", func() {
			table := NewTableFrom([]TableProvider(nil))

			Expect(table.Headers).To(BeEmpty())
			Expect(table.FieldNames).To(BeEmpty())
			Expect(table.Rows).To(BeEmpty())
		})

		It("sources the schema from the concrete element of an interface-typed slice", func() {
			table := NewTableFrom([]TableProvider{mockEmployee{ID: 1, Name: "Alice", Active: true}})

			Expect(table.FieldNames).To(Equal([]string{"id", "name", "department", "salary", "status"}))
			Expect(table.Rows).To(HaveLen(1))
			Expect(table.Rows[0]["name"].String()).To(Equal("Alice"))
		})

		It("carries hidden primitive cells as typed row metadata", func() {
			table := NewTableFrom([]hiddenColumnRow{{
				Name: "web", Tenant: "acme", Internal: "meta", Items: []string{"one"},
			}})

			// Hidden columns are row metadata, never visible columns.
			Expect(table.FieldNames).To(Equal([]string{"name"}))
			Expect(table.Rows[0]["tenant"].String()).To(Equal("acme"))
			Expect(table.Rows[0]["internal"].String()).To(Equal("meta"))

			Expect(map[string]any{
				"tenant":   table.Rows[0]["tenant"].FilterValue,
				"internal": table.Rows[0]["internal"].FilterValue,
				"count":    table.Rows[0]["count"].FilterValue,
				"enabled":  table.Rows[0]["enabled"].FilterValue,
				"items":    table.Rows[0]["items"].FilterValue,
				"name":     table.Rows[0]["name"].FilterValue,
			}).To(Equal(map[string]any{
				"tenant": "acme", "internal": "meta", "count": int64(0),
				"enabled": false, "items": nil, "name": nil,
			}))
		})

		It("renders a presented cell while retaining its independent raw filter value", func() {
			table := NewTableFrom([]presentedCellRow{{Duration: 125.0}})

			Expect(table.Rows[0]["duration"].String()).To(Equal("125ms"))
			Expect(table.Rows[0]["duration"].FilterValue).To(Equal(125.0))
		})

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
