package formatters

import (
	"bytes"

	"github.com/flanksource/clicky/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/xuri/excelize/v2"
)

type excelTableRow struct {
	ID   int
	Name string
}

func (r excelTableRow) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		{Name: "id", Label: "ID", Type: "number"},
		{Name: "name", Label: "Name"},
	}
}

func (r excelTableRow) Row() map[string]any {
	return map[string]any{"id": r.ID, "name": r.Name}
}

var _ = Describe("Excel formatter", func() {
	rows := []excelTableRow{{ID: 1, Name: "alpha"}, {ID: 2, Name: "beta"}}

	openBook := func(output string) *excelize.File {
		Expect(output).To(HavePrefix("PK\x03\x04"), "expected xlsx bytes")
		book, err := excelize.OpenReader(bytes.NewReader([]byte(output)))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(book.Close)
		return book
	}

	// --format excel used to reach ExcelFormatter.Format, which refused outright —
	// so a format the UI offers could not be produced at all off the manager path.
	It("renders table providers to xlsx bytes", func() {
		output, err := NewFormatManager().FormatWithOptions(FormatOptions{Format: "excel"}, rows)
		Expect(err).NotTo(HaveOccurred())

		book := openBook(output)
		Expect(book.GetCellValue("Sheet1", "A1")).To(Equal("ID"))
		Expect(book.GetCellValue("Sheet1", "B1")).To(Equal("Name"))
		Expect(book.GetCellValue("Sheet1", "A2")).To(Equal("1"))
		Expect(book.GetCellValue("Sheet1", "B3")).To(Equal("beta"))
	})

	It("accepts the xlsx alias", func() {
		output, err := NewFormatManager().FormatWithOptions(FormatOptions{Format: "xlsx"}, rows)
		Expect(err).NotTo(HaveOccurred())
		openBook(output)
	})

	// Every table-shaped PrettyData has a nil Schema (TryTypedValue sets only
	// TypedValue.Table), which is exactly what the old guard rejected.
	It("formats a PrettyData that carries a table but no schema", func() {
		data, err := ToPrettyData(rows)
		Expect(err).NotTo(HaveOccurred())
		Expect(data.Schema).To(BeNil())
		Expect(data.FirstTable()).NotTo(BeNil())

		output, err := NewExcelFormatter().FormatPrettyData(data)
		Expect(err).NotTo(HaveOccurred())
		Expect(openBook(output).GetCellValue("Sheet1", "A1")).To(Equal("ID"))
	})

	It("still refuses data with no table in it", func() {
		_, err := NewExcelFormatter().FormatPrettyData(nil)
		Expect(err).To(HaveOccurred())
	})
})
