package clicky

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/flanksource/clicky/formatters"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type sinkViews struct{}

type failingSinkWriter struct{}

func (failingSinkWriter) WriteFormat(output io.Writer, _ FormatOptions) error {
	if _, err := io.WriteString(output, "partial data"); err != nil {
		return err
	}
	return errors.New("render failed")
}

type replayableWriter struct {
	formats []string
	closed  bool
}

func (writer *replayableWriter) WriteFormat(output io.Writer, opts FormatOptions) error {
	writer.formats = append(writer.formats, opts.Format)
	_, err := io.WriteString(output, opts.Format+"\n")
	return err
}

func (writer *replayableWriter) Close() error {
	writer.closed = true
	return nil
}

func (sinkViews) FormatView(opts FormatOptions) (any, error) {
	if opts.Format == "json" {
		return map[string]any{"rows": []map[string]any{{"name": "policy"}}}, nil
	}
	return []map[string]any{{"name": "policy"}}, nil
}

var _ = ginkgo.Describe("format sinks", func() {
	ginkgo.It("uses an independent view for every format", func() {
		dir := ginkgo.GinkgoT().TempDir()
		jsonPath := filepath.Join(dir, "rows.json")
		csvPath := filepath.Join(dir, "rows.csv")
		err := PrintAndWriteSinks(sinkViews{}, FormatOptions{Sinks: []formatters.FormatSink{
			{Format: "json", File: jsonPath}, {Format: "csv", File: csvPath},
		}})
		Expect(err).NotTo(HaveOccurred())
		jsonData, err := os.ReadFile(jsonPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(jsonData)).To(ContainSubstring(`"rows"`))
		csvData, err := os.ReadFile(csvPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(csvData)).To(ContainSubstring("name"))
		Expect(string(csvData)).To(ContainSubstring("policy"))
	})

	ginkgo.It("returns file errors while continuing independent artifacts", func() {
		dir := ginkgo.GinkgoT().TempDir()
		target := filepath.Join(dir, "existing.csv")
		Expect(os.WriteFile(target, []byte("old data"), 0o600)).To(Succeed())
		badPath := filepath.Join(dir, "missing", "bad.json")
		err := PrintAndWriteSinks(sinkViews{}, FormatOptions{Sinks: []formatters.FormatSink{
			{Format: "json", File: badPath}, {Format: "csv", File: target},
		}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(badPath))
		data, readErr := os.ReadFile(target)
		Expect(readErr).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("policy"))
	})

	ginkgo.It("replays a stream once per sink and closes it after rendering", func() {
		dir := ginkgo.GinkgoT().TempDir()
		writer := &replayableWriter{}
		err := PrintAndWriteSinks(writer, FormatOptions{Sinks: []formatters.FormatSink{
			{Format: "json", File: filepath.Join(dir, "rows.json")},
			{Format: "csv", File: filepath.Join(dir, "rows.csv")},
		}})
		Expect(err).NotTo(HaveOccurred())
		Expect(writer.formats).To(Equal([]string{"json", "csv"}))
		Expect(writer.closed).To(BeTrue())
	})

	ginkgo.It("preserves an existing file when rendering fails", func() {
		target := filepath.Join(ginkgo.GinkgoT().TempDir(), "report.csv")
		Expect(os.WriteFile(target, []byte("original"), 0o600)).To(Succeed())
		Expect(FormatToFile(failingSinkWriter{}, FormatOptions{Format: "csv"}, target)).To(MatchError("render failed"))
		contents, err := os.ReadFile(target)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("original"))
	})
})
