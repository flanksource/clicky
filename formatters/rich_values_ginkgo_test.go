package formatters

import (
	"github.com/flanksource/clicky/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("rich link and badge values", func() {
	DescribeTable("preserves renderers inside structured input", func(wrap func(api.Link, api.LabelBadge) any) {
		link := api.NewLink("https://example.test/guide").AddText("Guide")
		badge := api.LabelBadge{Label: "Status", Value: "Ready"}
		output, err := NewFormatManager().FormatWithOptions(FormatOptions{Format: "markdown", NoColor: true}, wrap(link, badge))
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(SatisfyAll(ContainSubstring("[Guide](https://example.test/guide)"), ContainSubstring("**Status**: Ready"), Not(ContainSubstring("Href:")), Not(ContainSubstring("Icon:"))))
	},
		Entry("struct fields", func(link api.Link, badge api.LabelBadge) any {
			return struct{ Link, Badge any }{link, badge}
		}),
		Entry("map values", func(link api.Link, badge api.LabelBadge) any {
			return map[string]any{"link": link, "badge": badge}
		}),
		Entry("table cells", func(link api.Link, badge api.LabelBadge) any {
			return []map[string]any{{"link": link, "badge": badge}}
		}),
		Entry("schema-driven table cells", func(link api.Link, badge api.LabelBadge) any {
			return struct {
				Rows []struct{ Link, Badge any } `pretty:"table"`
			}{Rows: []struct{ Link, Badge any }{{link, badge}}}
		}),
	)
})
