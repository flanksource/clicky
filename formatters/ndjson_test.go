package formatters

import (
	"bytes"
	"context"
	"strings"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("ndjson", func() {
	Formatter := NewFormatManager()
	rows := []map[string]any{
		{"id": 1, "name": "alpha", "note": "a<b & c"},
		{"id": 2, "name": "beta", "note": "d>e"},
	}

	ginkgo.It("renders one compact JSON object per line", func() {
		out, err := Formatter.FormatWithOptions(FormatOptions{Format: "ndjson"}, rows)
		Expect(err).NotTo(HaveOccurred())

		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		Expect(lines).To(HaveLen(len(rows)))
		Expect(lines[0]).To(HavePrefix(`{"id":1,`))
	})

	// The CLI's --format ndjson and the server's streaming export must produce the
	// same bytes, or "the CLI has every format the UI has" is only true by name.
	ginkgo.It("is byte-identical to the streaming writer", func() {
		var streamed bytes.Buffer
		_, err := WriteTableStream(context.Background(), &streamed,
			&sliceRowIterator{rows: rows}, StreamOptions{Format: "ndjson"})
		Expect(err).NotTo(HaveOccurred())

		out, err := Formatter.FormatWithOptions(FormatOptions{Format: "ndjson"}, rows)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal(streamed.String()))
	})

	ginkgo.It("does not silently fall back to a pretty table", func() {
		out, err := Formatter.FormatWithOptions(FormatOptions{Format: "ndjson"}, rows)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(ContainSubstring("│"))
	})

	ginkgo.It("emits a single line for a value that is not a slice", func() {
		out, err := Formatter.FormatWithOptions(FormatOptions{Format: "ndjson"}, rows[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.Split(strings.TrimSuffix(out, "\n"), "\n")).To(HaveLen(1))
	})
})
