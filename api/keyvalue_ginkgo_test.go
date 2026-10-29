package api

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("rich key-value Markdown", func() {
	It("starts a one-item list below its field label", func() {
		value := List{Items: []Textable{NewText("Internal Team").Build()}}
		Expect(KeyValue("Approvers", value).Markdown()).To(Equal("**Approvers**:\n\n- Internal Team"))
		Expect(KeyValue("Approvers", CompactList([]string{"Internal Team"})).Markdown()).To(Equal("**Approvers**: Internal Team"))
	})

	It("preserves links and emphasis in map values", func() {
		values := map[string]Textable{"Instructions": NewLink("https://example.test/guide").AddText("Guide"), "Required": NewText("Yes").Bold().Build()}
		Expect(Map(values).Markdown()).To(Equal("**Instructions**: [Guide](https://example.test/guide), **Required**: **Yes**"))
	})

	It("separates multiline values from their keys and neighboring entries", func() {
		value := Text{}.Append("First paragraph").NewLine().NewLine().Append("Second paragraph")
		Expect(Map(map[string]Textable{"Body": value, "Owner": NewText("Alex").Build()}).Markdown()).To(Equal("**Body**:\n\nFirst paragraph\n\nSecond paragraph\n\n**Owner**: Alex"))
	})
})
