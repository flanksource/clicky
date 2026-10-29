package excel_test

import (
	"path/filepath"
	"reflect"

	"github.com/flanksource/clicky/flags"
	"github.com/flanksource/clicky/formatters/excel"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/xuri/excelize/v2"
)

func readWorksheetColumn(path, selector string) ([]string, error) {
	fields, err := flags.ParseStructFields(reflect.TypeFor[columnOpts]())
	Expect(err).NotTo(HaveOccurred())
	var opts columnOpts
	err = flags.PopulateFromRequest(reflect.ValueOf(&opts).Elem(), fields, map[string]string{"policies": "@" + path + ":" + selector}, nil)
	return opts.Policies, err
}

var _ = Describe("Excel worksheet column selectors", func() {
	var path string

	BeforeEach(func() {
		excel.Use()
		path = filepath.Join(GinkgoT().TempDir(), "issues.xlsx")
		book := excelize.NewFile()
		DeferCleanup(func() { Expect(book.Close()).To(Succeed()) })
		for index, sheet := range []string{"Sheet1", "Sheet2", "Sprint Backlog", "O'Brien", "Bang!Name", "2", "#2"} {
			if index > 0 {
				_, err := book.NewSheet(sheet)
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(book.SetSheetRow(sheet, "A1", &[]any{"Issue Key", "Priority!"})).To(Succeed())
			Expect(book.SetSheetRow(sheet, "A2", &[]any{sheet + "-first", "High"})).To(Succeed())
			Expect(book.SetSheetRow(sheet, "A3", &[]any{"", "Low"})).To(Succeed())
			Expect(book.SetSheetRow(sheet, "A4", &[]any{sheet + "-last"})).To(Succeed())
		}
		Expect(book.SaveAs(path)).To(Succeed())
	})

	DescribeTable("reads only the selected worksheet in row order, skipping empty cells",
		func(selector, sheet string) {
			Expect(readWorksheetColumn(path, selector)).To(Equal([]string{sheet + "-first", sheet + "-last"}))
		},
		Entry("default first sheet", "Issue Key", "Sheet1"),
		Entry("sheet name", "Sheet2!Issue Key", "Sheet2"),
		Entry("case-insensitive name and header", "sheet2!issue key", "Sheet2"),
		Entry("quoted sheet name with spaces", "'Sprint Backlog'!Issue Key", "Sprint Backlog"),
		Entry("sheet name with spaces inside the shell argument", "Sprint Backlog!Issue Key", "Sprint Backlog"),
		Entry("escaped apostrophe", "'O''Brien'!Issue Key", "O'Brien"),
		Entry("quoted exclamation mark", "'Bang!Name'!Issue Key", "Bang!Name"),
		Entry("second sheet by position", "#2!Issue Key", "Sheet2"),
		Entry("first sheet by position", "#1!Issue Key", "Sheet1"),
		Entry("last sheet by position", "#7!Issue Key", "#2"),
		Entry("numeric sheet name", "2!Issue Key", "2"),
		Entry("quoted index-like sheet name", "'#2'!Issue Key", "#2"),
	)

	It("preserves exclamation marks in the column header after the worksheet selector", func() {
		Expect(readWorksheetColumn(path, "Sheet2!Priority!")).To(Equal([]string{"High", "Low"}))
	})

	DescribeTable("rejects an invalid selector without reading another worksheet",
		func(selector, message string) {
			_, err := readWorksheetColumn(path, selector)
			Expect(err).To(MatchError(ContainSubstring(message)))
		},
		Entry("unknown worksheet", "Missing!Issue Key", `worksheet "Missing" not found`),
		Entry("missing column", "Sheet2!Missing", `column "Missing" not found`),
		Entry("index out of range", "#99!Issue Key", "worksheet index 99 out of range"),
		Entry("zero index", "#0!Issue Key", "worksheet index must be a positive integer"),
		Entry("negative index", "#-1!Issue Key", "worksheet index must be a positive integer"),
		Entry("nonnumeric index", "#two!Issue Key", "worksheet index must be a positive integer"),
		Entry("empty worksheet", "!Issue Key", "worksheet name is empty"),
		Entry("empty quoted worksheet", "''!Issue Key", "worksheet name is empty"),
		Entry("empty column", "Sheet2!", "column name is empty"),
		Entry("whitespace column", "Sheet2!  ", "column name is empty"),
		Entry("unclosed quote", "'Sprint Backlog!Issue Key", "unterminated quoted worksheet name"),
		Entry("missing separator after quote", "'Sheet2'Issue Key", "expected ! after quoted worksheet name"),
	)
})
