package api_test

import (
	"github.com/flanksource/clicky/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Full Markdown table cells", func() {
	It("preserves text transforms and emphasis while ignoring width limits", func() {
		table := api.NewEmptyTable([]api.ColumnDef{{Name: "value"}})
		table.Rows = []api.TableRow{{"value": api.NewTypedValue(api.Text{Content: "Example complete", Style: "uppercase max-w-[5] line-through"})}}
		Expect(api.RenderMarkdown(table, api.MarkdownOptions{NoColor: true})).To(ContainSubstring("~~EXAMPLE COMPLETE~~"))
	})
})
