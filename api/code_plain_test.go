package api

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("plainCodeANSI", func() {
	DescribeTable("renders code without syntax highlighting",
		func(code Code, want string) {
			Expect(plainCodeANSI(code)).To(Equal(want))
		},
		Entry("empty code renders nothing", Code{Language: "go"}, ""),
		Entry("known language passes through unchanged",
			Code{Content: "func main() {}", Language: "go"}, "func main() {}"),
		Entry("unknown language passes through unchanged",
			Code{Content: "some code", Language: "unknownlang"}, "some code"),
		Entry("multi-line keeps indentation and newlines",
			Code{Content: "if x {\n\treturn <y>\n}\n", Language: "go"}, "if x {\n\treturn <y>\n}\n"),
	)

	It("formats properties the same way the native renderer does", func() {
		code := Code{Content: "# comment\nkey=value", Language: "properties"}
		Expect(plainCodeANSI(code)).To(Equal(formatProperties(code.Content).ANSI()))
	})
})

var _ = Describe("plainCodeHTML", func() {
	DescribeTable("wraps escaped code in a pre/code block",
		func(code Code, want string) {
			Expect(plainCodeHTML(code)).To(Equal(want))
		},
		Entry("empty code renders nothing", Code{Language: "go"}, ""),
		Entry("language becomes a language-* class",
			Code{Content: "x := 1", Language: "go"},
			`<pre><code class="language-go">x := 1</code></pre>`),
		Entry("missing language omits the class",
			Code{Content: "plain"},
			`<pre><code>plain</code></pre>`),
		Entry("escapes < > & \" and '",
			Code{Content: `<a href="x">Tom & 'Jerry'</a>`, Language: "html"},
			`<pre><code class="language-html">&lt;a href=&#34;x&#34;&gt;Tom &amp; &#39;Jerry&#39;&lt;/a&gt;</code></pre>`),
		Entry("escapes a hostile language name inside the class attribute",
			Code{Content: "x", Language: `go" onclick="y`},
			`<pre><code class="language-go&#34; onclick=&#34;y">x</code></pre>`),
		Entry("multi-line keeps newlines inside the block",
			Code{Content: "line1\n  line2\n", Language: "yaml"},
			"<pre><code class=\"language-yaml\">line1\n  line2\n</code></pre>"),
	)

	It("formats properties the same way the native renderer does", func() {
		code := Code{Content: "# comment\nkey=<value>", Language: "conf"}
		Expect(plainCodeHTML(code)).To(Equal(formatProperties(code.Content).HTML()))
	})
})
