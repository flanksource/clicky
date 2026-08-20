package formatters

import (
	"strings"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type toonRow struct {
	ID     int     `json:"id"`
	Name   string  `json:"name,omitempty"`
	Region *string `json:"region"`
}

var _ = ginkgo.Describe("toon", func() {
	Formatter := NewFormatManager()
	rows := []toonRow{{ID: 1, Name: "alpha"}, {ID: 2, Name: "beta"}}

	// The tabular block is the whole point of TOON: naming the fields once
	// instead of once per row is where the token saving over JSON comes from.
	ginkgo.It("renders uniform rows as a single tabular block", func() {
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
	ginkgo.It("names fields the way --format json does", func() {
		out, err := Formatter.FormatWithOptions(FormatOptions{Format: "toon"}, rows)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(ContainSubstring("omitempty"))
		Expect(out).NotTo(ContainSubstring("ID"))
	})

	ginkgo.It("indents nested objects under their key", func() {
		out, err := Formatter.FormatWithOptions(FormatOptions{Format: "toon"},
			map[string]any{"tags": map[string]any{"env": "prod"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal("tags:\n  env: prod\n"))
	})

	ginkgo.It("renders an empty collection as a zero-length array", func() {
		out, err := Formatter.FormatWithOptions(FormatOptions{Format: "toon"}, []toonRow{})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal("[0]:\n"))
	})

	ginkgo.It("does not silently fall back to a pretty table", func() {
		out, err := Formatter.FormatWithOptions(FormatOptions{Format: "toon"}, rows)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(ContainSubstring("│"))
	})

	// --format validates against knownFormats before anything renders, so a
	// format the manager understands but the spec parser rejects is unreachable.
	ginkgo.It("is accepted by the --format spec parser", func() {
		opts := FormatOptions{Format: "toon"}
		Expect(opts.ParseFormatSpec()).To(Succeed())
		Expect(opts.Sinks).To(Equal([]FormatSink{{Format: "toon"}}))

		fileOpts := FormatOptions{Format: "pretty,toon=out.toon"}
		Expect(fileOpts.ParseFormatSpec()).To(Succeed())
		Expect(fileOpts.Sinks).To(ContainElement(FormatSink{Format: "toon", File: "out.toon"}))
	})

	ginkgo.It("is listed in the --format help", func() {
		Expect(FormatSpecHelp).To(ContainSubstring("toon"))
	})
})
