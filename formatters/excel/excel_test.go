package excel_test

import (
	"bytes"
	"testing"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/formatters/excel"
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

func openBook(output string) *excelize.File {
	GinkgoHelper()
	Expect(output).To(HavePrefix("PK\x03\x04"), "expected xlsx bytes")
	book, err := excelize.OpenReader(bytes.NewReader([]byte(output)))
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(book.Close)
	return book
}

var _ = Describe("Excel formatter", func() {
	rows := []excelTableRow{{ID: 1, Name: "alpha"}, {ID: 2, Name: "beta"}}

	Context("without excel.Use()", func() {
		BeforeEach(func() {
			formatters.UnregisterFormatter("excel")
		})

		It("explains how to register the format", func() {
			_, err := formatters.NewFormatManager().FormatWithOptions(formatters.FormatOptions{Format: "excel"}, rows)
			Expect(err).To(MatchError(ContainSubstring("excel.Use() from github.com/flanksource/clicky/formatters/excel")))
		})
	})

	Context("with excel.Use()", func() {
		BeforeEach(excel.Use)

		// --format excel used to reach ExcelFormatter.Format, which refused outright —
		// so a format the UI offers could not be produced at all off the manager path.
		It("renders table providers to xlsx bytes", func() {
			output, err := formatters.NewFormatManager().FormatWithOptions(formatters.FormatOptions{Format: "excel"}, rows)
			Expect(err).NotTo(HaveOccurred())

			book := openBook(output)
			Expect(book.GetCellValue("Sheet1", "A1")).To(Equal("ID"))
			Expect(book.GetCellValue("Sheet1", "B1")).To(Equal("Name"))
			Expect(book.GetCellValue("Sheet1", "A2")).To(Equal("1"))
			Expect(book.GetCellValue("Sheet1", "B3")).To(Equal("beta"))
		})

		It("renders an api.TextTable", func() {
			output, err := formatters.NewFormatManager().FormatWithOptions(formatters.FormatOptions{Format: "excel"}, api.NewTableFrom(rows))
			Expect(err).NotTo(HaveOccurred())
			Expect(openBook(output).GetCellValue("Sheet1", "A1")).To(Equal("ID"))
		})

		It("accepts the xlsx alias", func() {
			output, err := formatters.NewFormatManager().FormatWithOptions(formatters.FormatOptions{Format: "xlsx"}, rows)
			Expect(err).NotTo(HaveOccurred())
			openBook(output)
		})

		It("renders one worksheet per Workbook sheet through the format manager", func() {
			book := excel.Workbook{Sheets: []excel.Sheet{
				{Name: "first", Table: api.NewTableFrom(rows[:1])},
				{Name: "second", Table: api.NewTableFrom(rows[1:])},
			}}
			output, err := formatters.NewFormatManager().FormatWithOptions(formatters.FormatOptions{Format: "excel"}, book)
			Expect(err).NotTo(HaveOccurred())
			opened := openBook(output)
			Expect(opened.GetSheetList()).To(Equal([]string{"first", "second"}))
		})
	})

	// Every table-shaped PrettyData has a nil Schema (TryTypedValue sets only
	// TypedValue.Table), which is exactly what the old guard rejected.
	It("formats a PrettyData that carries a table but no schema", func() {
		data, err := formatters.ToPrettyData(rows)
		Expect(err).NotTo(HaveOccurred())
		Expect(data.Schema).To(BeNil())
		Expect(data.FirstTable()).NotTo(BeNil())

		output, err := excel.NewFormatter().FormatPrettyData(data)
		Expect(err).NotTo(HaveOccurred())
		Expect(openBook(output).GetCellValue("Sheet1", "A1")).To(Equal("ID"))
	})

	It("still refuses data with no table in it", func() {
		_, err := excel.NewFormatter().FormatPrettyData(nil)
		Expect(err).To(HaveOccurred())
	})
})

func TestExcel(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Excel Suite")
}
