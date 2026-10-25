package excel_test

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/flags"
	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/formatters/excel"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/xuri/excelize/v2"
)

type rowIterator struct {
	columns []api.ColumnDef
	rows    []map[string]any
	index   int
}

func (i *rowIterator) Columns() []api.ColumnDef { return i.columns }
func (i *rowIterator) Next() bool {
	if i.index >= len(i.rows) {
		return false
	}
	i.index++
	return true
}
func (i *rowIterator) Row() map[string]any { return i.rows[i.index-1] }
func (i *rowIterator) Err() error          { return nil }
func (i *rowIterator) Close() error        { return nil }

var _ = Describe("Excel streaming", func() {
	BeforeEach(excel.Use)

	stream := func(format string, columns []api.ColumnDef, rows []map[string]any) *excelize.File {
		var output bytes.Buffer
		_, err := formatters.WriteTableStream(context.Background(), &output, &rowIterator{columns: columns, rows: rows}, formatters.StreamOptions{Format: format})
		Expect(err).NotTo(HaveOccurred())
		return openBook(output.String())
	}

	It("streams rows through the xlsx alias", func() {
		book := stream("xlsx", []api.ColumnDef{{Name: "name"}, {Name: "count"}}, []map[string]any{{"name": "alpha", "count": 3}})
		Expect(book.GetCellValue("Sheet1", "A2")).To(Equal("alpha"))
	})

	It("keeps the raw value of a unit-formatted column", func() {
		book := stream("excel", []api.ColumnDef{{Name: "ratio", Type: "number", Unit: api.ColumnUnitPercentUnit}}, []map[string]any{{"ratio": 0.42}})
		Expect(book.GetCellValue("Sheet1", "A2")).To(Equal("0.42"))
	})

	It("escapes formulas but keeps numbers numeric", func() {
		book := stream("excel", []api.ColumnDef{{Name: "note"}, {Name: "delta", Type: "number"}}, []map[string]any{{"note": "=1+1", "delta": -5}})
		Expect(book.GetCellValue("Sheet1", "A2")).To(Equal("'=1+1"))
		Expect(book.GetCellValue("Sheet1", "B2")).To(Equal("-5"))
		Expect(book.GetCellType("Sheet1", "B2")).NotTo(BeElementOf(excelize.CellTypeInlineString, excelize.CellTypeSharedString))
	})
})

type columnOpts struct {
	Policies []string `flag:"policies" clicky:"rpc-file-read"`
}

var _ = Describe("@data.xlsx:Column flag values", func() {
	writeWorkbook := func() string {
		path := filepath.Join(GinkgoT().TempDir(), "policies.xlsx")
		book := excelize.NewFile()
		Expect(book.SetSheetRow("Sheet1", "A1", &[]any{"PolicyNumber", "Amount"})).To(Succeed())
		Expect(book.SetSheetRow("Sheet1", "A2", &[]any{"P001", 100})).To(Succeed())
		Expect(book.SetSheetRow("Sheet1", "A3", &[]any{"", 300})).To(Succeed())
		Expect(book.SetSheetRow("Sheet1", "A4", &[]any{"P003", 400})).To(Succeed())
		Expect(book.SaveAs(path)).To(Succeed())
		Expect(book.Close()).To(Succeed())
		return path
	}

	populate := func(path string) ([]string, error) {
		fields, err := flags.ParseStructFields(reflect.TypeOf(columnOpts{}))
		Expect(err).NotTo(HaveOccurred())
		var opts columnOpts
		err = flags.PopulateFromRequest(reflect.ValueOf(&opts).Elem(), fields, map[string]string{"policies": "@" + path + ":PolicyNumber"}, nil)
		return opts.Policies, err
	}

	It("reads the named column and skips blank cells", func() {
		excel.Use()
		Expect(populate(writeWorkbook())).To(Equal([]string{"P001", "P003"}))
	})

	It("fails with ErrNoSpreadsheetReader without excel.Use()", func() {
		flags.SetSpreadsheetReader(nil)
		_, err := populate(writeWorkbook())
		Expect(err).To(MatchError(flags.ErrNoSpreadsheetReader))
	})
})
