package formatters

import (
	"github.com/flanksource/clicky/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Non-terminal format manager", func() {
	DescribeTable("preserves binary bytes with NoColor on both formatting paths",
		func(format string) {
			const binary = "binary\x00\xff\x1b[31mpayload\x1b[0m"
			AddFormatCallback(FormatCallback{AfterFormat: func(_ any, _ *FormatManager, _ FormatOptions, _ any, _ string) string {
				return binary
			}})
			DeferCleanup(ClearFormatCallbacks)
			previous, registered := GetCustomFormatter(format)
			RegisterFormatter(format, func(any, FormatOptions) (string, error) { return binary, nil })
			DeferCleanup(func() {
				if registered {
					RegisterFormatter(format, previous)
				} else {
					UnregisterFormatter(format)
				}
			})
			manager := NewFormatManager(ManagerOptions{NoColor: true})
			options := FormatOptions{Format: format}
			output, err := manager.FormatWithContext(nil, options, "example")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(binary))
			output, err = manager.FormatWithSchemaContext(nil, &api.PrettyData{Original: "example"}, options)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(binary))
		},
		Entry("PDF", "pdf"), Entry("HTML PDF", "html-pdf"),
		Entry("Excel", "excel"), Entry("XLSX alias", "xlsx"),
	)

	It("constructs plain formatters before rendering and keeps output plain", func() {
		manager := NewFormatManager(ManagerOptions{NoColor: true})
		Expect(manager.prettyFormatter.NoColor).To(BeTrue())
		Expect(manager.prettyFormatter.Theme).To(Equal(api.NoTTYTheme()))
		for _, format := range []string{"json", "pretty", "tree", "markdown"} {
			output, err := manager.FormatWithOptions(FormatOptions{Format: format}, []struct{ Name string }{{"example"}})
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(ContainSubstring("\x1b["))
		}
	})
	It("removes ANSI returned by text values and plain strings", func() {
		manager := NewFormatManager(ManagerOptions{NoColor: true})
		for _, value := range []any{api.Text{Content: "example", Style: "text-red-500"}, "\x1b[31mexample\x1b[0m"} {
			output, err := manager.FormatWithOptions(FormatOptions{Format: "pretty"}, value)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(ContainSubstring("\x1b["))
		}
	})
})
