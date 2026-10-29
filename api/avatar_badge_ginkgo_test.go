package api

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("avatars and HTML badges", func() {
	It("caps avatar dimensions in HTML, Markdown and MDX", func() {
		avatar := Avatar{Name: "Alex", URL: "https://example.test/large-avatar.png"}
		for _, output := range []string{avatar.HTML(), avatar.Markdown()} {
			Expect(output).To(SatisfyAll(ContainSubstring("width: 24px"), ContainSubstring("height: 24px"), ContainSubstring("max-width: 24px"), ContainSubstring("max-height: 24px"), ContainSubstring("flex-shrink: 0")))
		}
		Expect(RenderMarkdown(avatar, MarkdownOptions{Dialect: DialectMDX})).To(SatisfyAll(ContainSubstring(`maxWidth: "24px"`), ContainSubstring(`maxHeight: "24px"`), ContainSubstring("flexShrink: 0"), Not(ContainSubstring("![]("))))
		avatar.Name = "Alex {example()} <Co>"
		Expect(RenderMarkdown(avatar, MarkdownOptions{Dialect: DialectMDX})).To(ContainSubstring("Alex &#123;example()&#125; &lt;Co&gt;"))
	})

	It("renders a sized avatar and name while keeping terminal and Slack labels compact", func() {
		avatar := Avatar{Name: "Alex & Co", URL: "https://example.test/avatar.png?size=24&v=1"}
		Expect(avatar.HTML()).To(SatisfyAll(ContainSubstring(`<img`), ContainSubstring(`width="24"`), ContainSubstring(`height="24"`), ContainSubstring("Alex &amp; Co"), ContainSubstring("size=24&amp;v=1")))
		Expect(avatar.Markdown()).To(SatisfyAll(ContainSubstring(`<img`), ContainSubstring(`width="24"`), ContainSubstring("Alex &amp; Co")))
		Expect(avatar.String()).To(Equal("Alex & Co"))
		Expect(avatar.ANSI()).To(Equal("Alex & Co"))
		Expect(RenderMarkdown(avatar, MarkdownOptions{Dialect: DialectSlack})).To(Equal("Alex &amp; Co"))
	})

	It("renders badge HTML with inline styling and escaped labels in GFM and HTML", func() {
		badge := Badge("To <Do>", "bg-slate-200 text-slate-800")
		for _, output := range []string{badge.HTML(), badge.Markdown(), RenderMarkdown(badge, MarkdownOptions{})} {
			Expect(output).To(SatisfyAll(ContainSubstring(`<span`), ContainSubstring(`class="badge`), ContainSubstring("border-radius:"), ContainSubstring("background-color:"), ContainSubstring("To &lt;Do&gt;")))
		}
		Expect(badge.String()).To(Equal("To <Do>"))
		Expect(RenderMarkdown(badge, MarkdownOptions{Dialect: DialectSlack})).To(Equal(`To \<Do\>`))
		Expect(RenderMarkdown(badge, MarkdownOptions{NoColor: true})).To(SatisfyAll(ContainSubstring(`<span`), ContainSubstring(`class="badge`), ContainSubstring("To &lt;Do&gt;"), Not(ContainSubstring("background-color:"))))
	})

	It("escapes image attributes without adding new attributes", func() {
		image := HtmlElement{Tag: "img", Attributes: map[string]string{"src": `https://example.test/avatar.png?name="Alex"&size=24`}}
		Expect(image.HTML()).To(Equal(`<img src="https://example.test/avatar.png?name=&quot;Alex&quot;&amp;size=24">`))
	})
})
