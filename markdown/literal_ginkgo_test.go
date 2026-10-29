package markdown_test

import (
	"github.com/flanksource/clicky/markdown"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("literal text serialization", func() {
	It("preserves literal HTML-looking text and Markdown punctuation through serialization", func() {
		const text = "For <customers> use *literal* &copy;"
		node := markdown.Node{Kind: "paragraph", Children: []markdown.Node{{Kind: "text", Text: text}}}
		const source = `For \<customers\> use \*literal\* &amp;copy;`
		Expect(node.Markdown()).To(Equal(source))
		document, err := markdown.ParseString(source)
		Expect(err).NotTo(HaveOccurred())
		Expect(document.String()).To(Equal(text))
		Expect(document.Markdown()).To(Equal(source))
		Expect(document.HTML()).To(Equal("<p>For &lt;customers&gt; use *literal* &amp;copy;</p>"))
	})
})
