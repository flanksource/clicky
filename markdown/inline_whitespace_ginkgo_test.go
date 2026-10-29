package markdown_test

import (
	"github.com/flanksource/clicky/markdown"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("semantic inline whitespace", func() {
	It("preserves explicit breaks and space-only text between formatted nodes", func() {
		node := markdown.Node{Kind: "paragraph", Children: []markdown.Node{
			{Kind: "strong", Children: []markdown.Node{{Kind: "text", Text: "First"}}},
			{Kind: "text", Text: " "}, {Kind: "text", Text: "item"}, {Kind: "linebreak"},
			{Kind: "text", Text: "Second item"},
		}}
		Expect(node.Markdown()).To(Equal("**First** item  \nSecond item"))
		Expect(node.HTML()).To(Equal("<p><strong>First</strong> item<br />Second item</p>"))
	})
})
