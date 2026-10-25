package formatters

import (
	"encoding/csv"
	"regexp"
	"strings"

	"github.com/flanksource/clicky/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type toonRow struct {
	ID     int     `json:"id"`
	Name   string  `json:"name,omitempty"`
	Region *string `json:"region"`
}

var _ = Describe("toon", func() {
	Formatter := NewFormatManager()
	rows := []toonRow{{ID: 1, Name: "alpha"}, {ID: 2, Name: "beta"}}

	// The tabular block is the whole point of TOON: naming the fields once
	// instead of once per row is where the token saving over JSON comes from.
	It("renders uniform rows as a single tabular block", func() {
		out, err := Formatter.FormatWithOptions(FormatOptions{Format: "toon"}, rows)
		Expect(err).NotTo(HaveOccurred())

		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		Expect(lines).To(HaveLen(len(rows) + 1))
		Expect(lines[0]).To(Equal("[2]{id,name,region}:"))
		Expect(lines[1]).To(Equal("  1,alpha,null"))
		Expect(lines[2]).To(Equal("  2,beta,null"))
	})

	// gotoon's own struct walker takes the whole json tag as the key and ignores
	// omitempty and MarshalJSON, so a struct has to reach it already decoded or
	// the output carries keys like "name,omitempty" that no consumer can read.
	It("names fields the way --format json does", func() {
		out, err := Formatter.FormatWithOptions(FormatOptions{Format: "toon"}, rows)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(ContainSubstring("omitempty"))
		Expect(out).NotTo(ContainSubstring("ID"))
	})

	It("indents nested objects under their key", func() {
		out, err := Formatter.FormatWithOptions(FormatOptions{Format: "toon"},
			map[string]any{"tags": map[string]any{"env": "prod"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal("tags:\n  env: prod\n"))
	})

	It("renders an empty collection as a zero-length array", func() {
		out, err := Formatter.FormatWithOptions(FormatOptions{Format: "toon"}, []toonRow{})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal("[0]:\n"))
	})

	It("does not silently fall back to a pretty table", func() {
		out, err := Formatter.FormatWithOptions(FormatOptions{Format: "toon"}, rows)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(ContainSubstring("│"))
	})

	// --format validates against knownFormats before anything renders, so a
	// format the manager understands but the spec parser rejects is unreachable.
	It("is accepted by the --format spec parser", func() {
		opts := FormatOptions{Format: "toon"}
		Expect(opts.ParseFormatSpec()).To(Succeed())
		Expect(opts.Sinks).To(Equal([]FormatSink{{Format: "toon"}}))

		fileOpts := FormatOptions{Format: "pretty,toon=out.toon"}
		Expect(fileOpts.ParseFormatSpec()).To(Succeed())
		Expect(fileOpts.Sinks).To(ContainElement(FormatSink{Format: "toon", File: "out.toon"}))
	})

	It("is listed in the --format help", func() {
		Expect(FormatSpecHelp).To(ContainSubstring("toon"))
	})
})

var gridjsTableID = regexp.MustCompile(`gridjs-table-\d+`)

func normalizeTableIDs(html string) string {
	return gridjsTableID.ReplaceAllString(html, "gridjs-table")
}

type textableRow struct {
	ID   int
	Name string
}

func (r textableRow) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		{Name: "id", Label: "ID", Type: "number"},
		{Name: "name", Label: "Name"},
	}
}

func (r textableRow) Row() map[string]any {
	return map[string]any{"id": r.ID, "name": r.Name}
}

var _ = Describe("Textable formatting", func() {
	table := api.NewTableFrom([]textableRow{{ID: 1, Name: "alpha"}, {ID: 2, Name: "beta"}})

	// api.TextTable is what every Table()/Pretty() producer hands the formatter, so
	// a format it cannot reach is a format the caller cannot use.
	DescribeTable("renders a TextTable in formats formatTextable does not implement",
		func(format string, assert func(string)) {
			output, err := NewFormatManager().FormatWithOptions(FormatOptions{Format: format}, table)
			Expect(err).NotTo(HaveOccurred())
			assert(output)
		},
		Entry("csv", "csv", func(output string) {
			records, err := csv.NewReader(strings.NewReader(output)).ReadAll()
			Expect(err).NotTo(HaveOccurred())
			Expect(records[0]).To(Equal([]string{"ID", "Name"}))
			Expect(records[2]).To(Equal([]string{"2", "beta"}))
		}),
		Entry("ndjson", "ndjson", func(output string) {
			Expect(strings.Count(strings.TrimSuffix(output, "\n"), "\n")).To(Equal(0))
			Expect(output).To(ContainSubstring("alpha"))
		}),
		Entry("toon", "toon", func(output string) {
			Expect(output).To(HavePrefix("[2]{"))
			Expect(output).To(ContainSubstring("alpha"))
		}),
		Entry("tree", "tree", func(output string) {
			Expect(output).NotTo(BeEmpty())
		}),
	)

	// The formats formatTextable does implement must keep going through it, or
	// widening the fall-through would quietly rewrite every existing output.
	DescribeTable("leaves formats formatTextable owns untouched",
		func(format string) {
			expected, err := formatTextable(table, FormatOptions{Format: format})
			Expect(err).NotTo(HaveOccurred())

			output, err := NewFormatManager().FormatWithOptions(FormatOptions{Format: format}, table)
			Expect(err).NotTo(HaveOccurred())
			// The Grid.js mount id is deliberately unique per render, so two renders of
			// the same table never match byte-for-byte; the rest must be identical.
			Expect(normalizeTableIDs(output)).To(Equal(normalizeTableIDs(expected)))
		},
		Entry("json", "json"),
		Entry("yaml", "yaml"),
		Entry("markdown", "markdown"),
		Entry("pretty", "pretty"),
		Entry("html", "html"),
		Entry("slack", "slack"),
	)
})
