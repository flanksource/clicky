package formatters

import (
	"github.com/flanksource/clicky/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type markdownPretty struct{}

func (markdownPretty) Pretty() api.Text {
	return api.Text{}.
		Append("muted", "text-muted").
		Append(" important", "font-bold text-green-600")
}

var _ = Describe("Markdown no-color rendering", func() {
	It("removes Clicky color spans from direct Textable output", func() {
		manager := NewFormatManager()
		output, err := manager.FormatWithOptions(
			FormatOptions{Format: "markdown", NoColor: true},
			markdownPretty{}.Pretty(),
		)

		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(Equal("muted** important**"))
		Expect(output).NotTo(ContainSubstring("<span"))
	})

	It("uses MarkdownFormatter.NoColor for Pretty output", func() {
		formatter := NewMarkdownFormatter()
		formatter.NoColor = true

		output, err := formatter.Format(markdownPretty{})

		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(Equal("muted** important**"))
		Expect(output).NotTo(ContainSubstring("<span"))
	})

	It("propagates no-color through schema-ordered PrettyData", func() {
		values := api.TypedMap{
			"status": api.NewTypedValue(api.Text{}.Append("ready", "text-green-600")),
		}
		data := &api.PrettyData{
			TypedValue: api.TypedValue{TypedMap: &values},
			Schema:     &api.PrettyObject{Fields: []api.PrettyField{{Name: "status", Label: "Status"}}},
		}

		output, err := NewMarkdownFormatter().FormatPrettyData(data, FormatOptions{NoColor: true})

		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(Equal("Status: ready"))
		Expect(output).NotTo(ContainSubstring("<span"))
	})

	It("propagates no-color through lists", func() {
		list := api.List{Items: []api.Textable{api.Text{}.Append("ready", "text-green-600")}}

		output, err := NewFormatManager().FormatWithOptions(
			FormatOptions{Format: "markdown", NoColor: true},
			list,
		)

		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(Equal("ready"))
		Expect(output).NotTo(ContainSubstring("<span"))
	})

	It("keeps color spans when no-color is disabled", func() {
		output, err := NewMarkdownFormatter().Format(markdownPretty{})

		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(ContainSubstring(`<span style="color: #6b7280">muted</span>`))
		Expect(output).To(ContainSubstring(`<span style="color: #16a34a">** important**</span>`))
	})
})
