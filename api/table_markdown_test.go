package api

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type markdownTableRow struct {
	value any
	width int
}

func (r markdownTableRow) Columns() []ColumnDef {
	column := Column("value").Label("Value")
	if r.width > 0 {
		column.MaxWidth(r.width)
	}
	return []ColumnDef{column.Build()}
}

func (r markdownTableRow) Row() map[string]any {
	return map[string]any{"value": r.value}
}

func markdownTableLines(rendered string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(rendered), "\n") {
		if strings.HasPrefix(line, "|") {
			lines = append(lines, line)
		}
	}
	return lines
}

var _ = Describe("TextTable Markdown rendering", func() {
	It("does not depend on terminal width", func() {
		value := strings.Repeat("a", 150)
		table := NewTableFrom([]markdownTableRow{{value: value}})

		previousWidth := terminalWidth.Swap(30)
		narrow := table.Markdown()
		terminalWidth.Store(240)
		wide := table.Markdown()
		terminalWidth.Store(previousWidth)

		Expect(narrow).To(Equal(wide))
		Expect(narrow).NotTo(ContainSubstring("↩"))
		Expect(markdownTableLines(narrow)).To(HaveLen(3))
	})

	It("uses an explicit column MaxWidth without breaking rich markup", func() {
		value := Text{}.Append("abcdefghijklmno", "text-green-600")
		rendered := NewTableFrom([]markdownTableRow{{value: value, width: 10}}).Markdown()

		Expect(rendered).To(ContainSubstring("abcdefghi…"))
		Expect(rendered).NotTo(ContainSubstring("<span sty"))
		Expect(rendered).NotTo(ContainSubstring("↩"))
		Expect(markdownTableLines(rendered)).To(HaveLen(3))
	})

	It("caps columns without MaxWidth at 200 visible characters", func() {
		value := strings.Repeat("b", 205)
		rendered := NewTableFrom([]markdownTableRow{{value: value}}).Markdown()

		Expect(rendered).To(ContainSubstring(strings.Repeat("b", 199) + "…"))
		Expect(rendered).NotTo(ContainSubstring(strings.Repeat("b", 200)))
		Expect(markdownTableLines(rendered)).To(HaveLen(3))
	})

	It("preserves complete styles for cells within their width", func() {
		value := Text{}.Append("ready", "text-green-600")
		rendered := NewTableFrom([]markdownTableRow{{value: value}}).Markdown()

		Expect(rendered).To(ContainSubstring(`<span style="color: #16a34a">ready</span>`))
	})

	It("encodes cell newlines without creating additional rows", func() {
		value := "first\nsecond\r\n\rthird"
		rendered := NewTableFrom([]markdownTableRow{{value: value}}).Markdown()

		Expect(rendered).To(ContainSubstring("first<br>second<br><br>third"))
		Expect(markdownTableLines(rendered)).To(HaveLen(3))
	})
})
