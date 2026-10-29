package markdown_test

import (
	"github.com/flanksource/clicky/markdown"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("code delimiter preservation", func() {
	It("round trips nested code fences and inline backticks", func() {
		const source = "````markdown\n```go\nrun()\n```\n````\n\nUse ``a`b`` and `` `edge` ``."
		doc, err := markdown.ParseString(source)
		Expect(err).NotTo(HaveOccurred())
		restored, err := markdown.ParseString(doc.Markdown())
		Expect(err).NotTo(HaveOccurred())
		Expect(restored.HTML()).To(Equal(doc.HTML()))
		Expect(restored.String()).To(Equal(doc.String()))
	})
})
