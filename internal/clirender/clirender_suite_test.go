package clirender

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestClirender(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CLI Render")
}

var _ = DescribeTable("resolveInputOutputPath",
	func(outputPattern, sourceName, format, want string) {
		Expect(resolveInputOutputPath(outputPattern, sourceName, format)).To(Equal(want))
	},
	Entry("wildcard without extension gets the format extension", "out/*", "data.json", "yaml", "out/data.yaml"),
	Entry("wildcard with explicit extension is preserved", "out/*.txt", "data.json", "yaml", "out/data.txt"),
	Entry("wildcard with stdin source uses stdin base", "*", "", "csv", "stdin.csv"),
	Entry("directory pattern joins base plus extension", "out", "data.json", "json", filepath.Join("out", "data.json")),
	Entry("explicit file path is returned unchanged", "out/report.pdf", "data.json", "json", "out/report.pdf"),
)

func expectDetectedInputFormat(data, sourceName, want string) {
	Expect(detectInputFormat([]byte(data), sourceName)).To(Equal(want))
}

var _ = DescribeTable("detectInputFormat", expectDetectedInputFormat,
	Entry("file extension wins over content", `{"name":"demo"}`, "notes.md", "markdown"),
	Entry("blank input is text", "  \n", "", "text"),
	Entry("valid JSON is json", `{"name":"demo"}`, "", "json"),
	Entry("front matter is markdown", "---\ntitle: demo\n---\nbody", "", "markdown"),
	Entry("heading is markdown", "# Title\nbody", "", "markdown"),
	Entry("ordered list is markdown", "1. first\n2. second", "", "markdown"),
	Entry("YAML mapping is yaml", "name: demo\nreplicas: 2", "", "yaml"),
	Entry("plain prose is text", "just a sentence", "", "text"),
)

func expectNormalizedInputFormat(format, want string) {
	Expect(normalizeInputFormat(format)).To(Equal(want))
}

var _ = DescribeTable("normalizeInputFormat", expectNormalizedInputFormat,
	Entry("empty means auto", "", "auto"),
	Entry("md alias is markdown", " MD ", "markdown"),
	Entry("yml alias is yaml", "yml", "yaml"),
	Entry("plain alias is text", "plain", "text"),
)

var _ = It("rejects an unsupported input format", func() {
	_, err := normalizeInputFormat("toml")
	Expect(err).To(MatchError(ContainSubstring(`unsupported --input-format "toml"`)))
})
