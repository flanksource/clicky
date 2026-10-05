package formatters_test

import (
	"fmt"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/formatters"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type llmRecord struct {
	Index int
	Value string
}

func (r llmRecord) Columns() []api.ColumnDef {
	return []api.ColumnDef{api.Column("value").MaxWidth(5).Build()}
}
func (r llmRecord) Row() map[string]any {
	return map[string]any{"value": api.NewLink(fmt.Sprintf("/records/%d", r.Index)).Text(r.Value)}
}
func (r llmRecord) RowDetail() api.Textable { panic("list preview must not expand row details") }

var _ = Describe("LLM list format", func() {
	DescribeTable("bounds rows and reports the displayed window", func(count int) {
		rows := make([]llmRecord, count)
		for i := range rows {
			rows[i] = llmRecord{Index: i, Value: fmt.Sprintf("record-%03d", i)}
		}
		out, err := formatters.NewFormatManager().FormatWithOptions(formatters.FormatOptions{Format: "llm"}, entity.NewPagedResult(rows, 100, 50, int64(50+count)))
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring(fmt.Sprintf("Shown: %d", min(count, 25))))
		Expect(out).To(ContainSubstring(fmt.Sprintf("Total: %d", 50+count)))
		Expect(out).To(ContainSubstring("Use --json for complete fields and all returned rows."))
		if count > 25 {
			Expect(out).To(ContainSubstring("--offset 75 --limit 25"))
			Expect(out).NotTo(ContainSubstring("record-025"))
		} else {
			Expect(out).To(ContainSubstring("Has more: false"))
		}
	}, Entry("empty", 0), Entry("24", 24), Entry("25", 25), Entry("26", 26))
	It("keeps full Unicode links, pipes and multiline cells", func() {
		value := strings.Repeat("長い記述", 80) + "|first\nlast"
		out, err := formatters.NewFormatManager().FormatWithOptions(formatters.FormatOptions{Format: "llm"}, []llmRecord{{Index: 1, Value: value}})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring(strings.Repeat("長い記述", 80) + "\\|first<br>last](/records/1)"))
		Expect(out).To(ContainSubstring("Total: unknown"))
	})
	It("keeps column headers for an empty typed provider result", func() {
		out, err := formatters.NewFormatManager().FormatWithOptions(formatters.FormatOptions{Format: "llm"}, []llmRecord{})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("| VALUE |"))
		Expect(out).To(ContainSubstring("Shown: 0"))
	})
	It("reports an unknown continuation when a full buffered page has no total", func() {
		rows := make([]llmRecord, 25)
		for i := range rows {
			rows[i] = llmRecord{Index: i, Value: fmt.Sprintf("record-%03d", i)}
		}
		out, err := formatters.NewFormatManager().FormatWithOptions(formatters.FormatOptions{Format: "llm"}, entity.NewPagedResult(rows, 25, 0, -1))
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("Total: unknown; Has more: unknown"))
		Expect(out).To(ContainSubstring("--offset 25 --limit 25"))
	})
	It("parses llm stdout and independent JSON file sinks", func() {
		opts := formatters.FormatOptions{Format: "llm,json=records.json"}
		Expect(opts.ParseFormatSpec()).To(Succeed())
		Expect(opts.Sinks).To(Equal([]formatters.FormatSink{{Format: "llm"}, {Format: "json", File: "records.json"}}))
	})
	It("does not invent continuation flags for an unpageable list", func() {
		rows := make([]llmRecord, 26)
		for i := range rows {
			rows[i] = llmRecord{Index: i, Value: fmt.Sprintf("record-%03d", i)}
		}
		out, err := formatters.NewFormatManager().FormatWithOptions(formatters.FormatOptions{Format: "llm"}, rows)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("Has more: true"))
		Expect(out).To(ContainSubstring("Paging is unavailable"))
		Expect(out).To(ContainSubstring("Use --json for complete fields and all returned rows."))
		Expect(out).NotTo(ContainSubstring("--offset"))
	})
})
