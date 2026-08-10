package formatters

import (
	"bytes"
	"encoding/csv"
	"strings"

	"github.com/flanksource/clicky/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/xuri/excelize/v2"
)

var _ = Describe("Textable formatting", func() {
	table := api.NewTableFrom([]excelTableRow{{ID: 1, Name: "alpha"}, {ID: 2, Name: "beta"}})

	// api.TextTable is what every Table()/Pretty() producer hands the formatter, so
	// a format it cannot reach is a format the caller cannot use.
	DescribeTable("renders a TextTable in formats formatTextable does not implement",
		func(format string, assert func(string)) {
			output, err := NewFormatManager().FormatWithOptions(FormatOptions{Format: format}, table)
			Expect(err).NotTo(HaveOccurred())
			assert(output)
		},
		Entry("csv", "csv", func(output string) {
			records, err := csv.NewReader(strings.NewReader(output)).ReadAll()
			Expect(err).NotTo(HaveOccurred())
			Expect(records[0]).To(Equal([]string{"ID", "Name"}))
			Expect(records[2]).To(Equal([]string{"2", "beta"}))
		}),
		Entry("excel", "excel", func(output string) {
			book, err := excelize.OpenReader(bytes.NewReader([]byte(output)))
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(book.Close)
			Expect(book.GetCellValue("Sheet1", "A1")).To(Equal("ID"))
		}),
		Entry("ndjson", "ndjson", func(output string) {
			Expect(strings.Count(strings.TrimSuffix(output, "\n"), "\n")).To(Equal(0))
			Expect(output).To(ContainSubstring("alpha"))
		}),
		Entry("tree", "tree", func(output string) {
			Expect(output).NotTo(BeEmpty())
		}),
	)

	// The formats formatTextable does implement must keep going through it, or
	// widening the fall-through would quietly rewrite every existing output.
	DescribeTable("leaves formats formatTextable owns untouched",
		func(format string) {
			expected, err := formatTextable(table, FormatOptions{Format: format})
			Expect(err).NotTo(HaveOccurred())

			output, err := NewFormatManager().FormatWithOptions(FormatOptions{Format: format}, table)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(expected))
		},
		Entry("json", "json"),
		Entry("yaml", "yaml"),
		Entry("markdown", "markdown"),
		Entry("pretty", "pretty"),
		Entry("html", "html"),
		Entry("slack", "slack"),
	)
})
