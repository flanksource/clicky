package api

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("semantic render parity", func() {
	It("preserves links and badges before scalar schema conversion", func() {
		for _, value := range []Textable{NewLink("https://example.test").AddText("Guide"), LabelBadge{Label: "Status", Value: "Ready"}} {
			parsed, err := (PrettyField{Name: "value", Type: "string"}).Parse(value)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed.Text).To(Equal(value))
		}
	})

	It("renders noninteractive tables as HTML elements", func() {
		table := TextTable{Headers: TextList{NewText("Name").Build()}, Rows: []TableRow{{"Name": NewTypedValue("Report")}}}
		Expect(table.HTML()).To(SatisfyAll(ContainSubstring("<table"), ContainSubstring("Report"), Not(ContainSubstring("<script>"))))
		table.RowDetail = []Textable{CodeBlock("text", "Report details")}
		Expect(table.HTML()).To(SatisfyAll(ContainSubstring("Report details"), Not(ContainSubstring("<template")), Not(ContainSubstring("x-data"))))
		table.RowDetail = nil
		table.Interactive = true
		Expect(table.HTML()).To(ContainSubstring("new gridjs.Grid"))
	})

	It("escapes literal text without changing its plain value", func() {
		text := NewText("For <customers> use *literal* &copy;").Build()
		Expect(text.String()).To(Equal("For <customers> use *literal* &copy;"))
		Expect(text.HTML()).To(Equal("For &lt;customers&gt; use *literal* &amp;copy;"))
		Expect(text.Markdown()).To(Equal(`For \<customers\> use \*literal\* &amp;copy;`))
	})

	It("transforms its own content while preserving child links", func() {
		text := NewText("parent").Uppercase().Build().Add(NewLink("https://example.test/Mixed").AddText("Guide"))
		Expect(text.HTML()).To(ContainSubstring(`PARENT</span><a href="https://example.test/Mixed">Guide</a>`))
		Expect(text.Markdown()).To(Equal("PARENT[Guide](https://example.test/Mixed)"))
	})

	It("preserves rich map values and escapes their literal keys", func() {
		value := Map(map[string]Textable{"Guide [current]": NewLink("https://example.test").AddText("Read *this*")})
		Expect(value.HTML()).To(ContainSubstring(`<a href="https://example.test">Read *this*</a>`))
		Expect(value.HTML()).NotTo(ContainSubstring("\x1b"))
		Expect(value.Markdown()).To(Equal(`**Guide \[current\]**: [Read \*this\*](https://example.test)`))
	})

	It("keeps multiline content inside its list item", func() {
		list := List{Items: []Textable{NewText("First\ncontinued").Build(), NewText("Second").Build()}}
		Expect(list.Markdown()).To(Equal("- First\n  continued\n- Second"))
		Expect(list.String()).To(Equal("- First\n  continued\n- Second"))
		Expect(list.HTML()).To(Equal("<ul><li>First\ncontinued</li><li>Second</li></ul>"))
		list.Ordered = true
		Expect(list.Markdown()).To(Equal("1. First\n   continued\n2. Second"))
		Expect(list.HTML()).To(HavePrefix("<ol>"))
		list.Unstyled = true
		Expect(list.Markdown()).To(Equal("First\ncontinued\nSecond"))
		Expect(list.HTML()).NotTo(ContainSubstring("<ol>"))
	})

	It("keeps a compact scalar list inline in every format", func() {
		list := CompactList([]string{"alpha", "beta"})
		Expect(list.String()).To(Equal("alpha, beta"))
		Expect(list.Markdown()).To(Equal("alpha, beta"))
		Expect(list.HTML()).To(Equal("alpha, beta"))
	})

	It("keeps code fences longer than embedded backticks", func() {
		Expect(CodeBlock("markdown", "```go\nrun()\n```").Markdown()).To(Equal("````markdown\n```go\nrun()\n```\n````"))
	})

	It("escapes link destinations containing spaces and unbalanced parentheses", func() {
		Expect(NewLink("https://example.test/a)b c").AddText("Guide").Markdown()).To(Equal("[Guide](https://example.test/a%29b%20c)"))
	})
})
