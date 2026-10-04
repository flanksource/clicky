package clicky_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/formatters"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
)

var _ = Describe("CLI LLM formats", func() {
	It("binds --llm as the first-class format alias", func() {
		opts := formatters.FormatOptions{}
		flags := pflag.NewFlagSet("example", pflag.ContinueOnError)
		formatters.BindPFlags(flags, &opts)
		Expect(flags.Parse([]string{"--llm"})).To(Succeed())
		Expect(opts.ResolveFormat()).To(Equal("llm"))
	})
	It("keeps a full JSON artifact while rendering a bounded preview", func() {
		rows := make([]map[string]string, 26)
		for i := range rows {
			rows[i] = map[string]string{"name": fmt.Sprintf("item-%03d", i)}
		}
		page := clicky.NewPagedResult(rows, 100, 0, 26)
		out, err := clicky.Format(page, clicky.FormatOptions{LLM: true})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("item-024"))
		Expect(out).NotTo(ContainSubstring("item-025"))
		Expect(os.MkdirAll(".tmp", 0700)).To(Succeed())
		dir, err := os.MkdirTemp(".tmp", "llm-format-")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, dir)
		path := filepath.Join(dir, "records.json")
		Expect(clicky.FormatToFile(page, clicky.FormatOptions{Format: "json"}, path)).To(Succeed())
		contents, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		var decoded []map[string]string
		Expect(json.Unmarshal(contents, &decoded)).To(Succeed())
		Expect(decoded).To(Equal(rows))
	})
})
