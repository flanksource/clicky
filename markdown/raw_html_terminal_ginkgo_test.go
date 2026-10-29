package markdown_test

import (
	"github.com/flanksource/clicky/markdown"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HTML-backed semantic text", func() {
	It("uses explicit plain text in terminal output while preserving rich formats", func() {
		node := markdown.Node{Kind: "raw-html", Text: "Amount < limit", Source: "<u>Amount \\< limit</u>", RawHTML: "<u>Amount &lt; limit</u>"}
		Expect(node.ANSI()).To(Equal("Amount < limit"))
		Expect(node.String()).To(Equal("Amount < limit"))
		Expect(node.Markdown()).To(Equal("<u>Amount \\< limit</u>"))
		Expect(node.HTML()).To(Equal("<u>Amount &lt; limit</u>"))
	})

	It("renders parsed HTML as plain terminal text and preserves rich output", func() {
		for _, example := range []struct{ Source, Markdown string }{
			{"<u>Amount &lt; limit</u>", "<u>Amount \\< limit</u>"},
			{"<div>Amount &lt; limit</div>", "<div>Amount &lt; limit</div>"},
		} {
			doc, err := markdown.ParseString(example.Source)
			Expect(err).NotTo(HaveOccurred())
			Expect(doc.String()).To(Equal("Amount < limit"))
			Expect(doc.ANSI()).To(Equal("Amount < limit"))
			Expect(doc.Markdown()).To(ContainSubstring(example.Markdown))
			Expect(doc.HTML()).To(ContainSubstring(example.Source))
		}
	})
})
