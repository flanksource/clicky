package formatters

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("format specification preflight", func() {
	It("accepts registered custom formats for stdout and file sinks", func() {
		RegisterFormatter("report-format", func(any, FormatOptions) (string, error) { return "report", nil })
		defer UnregisterFormatter("report-format")
		opts := FormatOptions{Format: "report-format,report-format=report.html"}
		Expect(opts.ParseFormatSpec()).To(Succeed())
		Expect(opts.Sinks).To(Equal([]FormatSink{{Format: "report-format"}, {Format: "report-format", File: "report.html"}}))
	})
	It("leaves no partial sinks after a bad entry", func() {
		opts := FormatOptions{Format: "json=rows.json,unknown"}
		Expect(opts.ParseFormatSpec()).To(MatchError(ContainSubstring("unknown format")))
		Expect(opts.Sinks).To(BeEmpty())
	})

	It("refuses duplicate file targets and conflicting flags", func() {
		opts := FormatOptions{Format: "json=rows.json,csv=" + filepath.Join(".", "rows.json")}
		Expect(opts.ParseFormatSpec()).To(MatchError(ContainSubstring("same output file")))
		conflicting := FormatOptions{Format: "csv", JSON: true}
		Expect(conflicting.ParseFormatSpec()).To(HaveOccurred())
		incompatible := FormatOptions{Format: "json", Tree: true}
		Expect(incompatible.ParseFormatSpec()).To(HaveOccurred())
		legacyIncompatible := FormatOptions{JSON: true, Tree: true}
		Expect(legacyIncompatible.ParseFormatSpec()).To(HaveOccurred())
	})

	It("uses --output as the file sink for the chosen format", func() {
		opts := FormatOptions{Format: "markdown", Output: "report.md"}
		Expect(opts.ParseFormatSpec()).To(Succeed())
		Expect(opts.Sinks).To(Equal([]FormatSink{{Format: "markdown", File: "report.md"}}))
	})
})
