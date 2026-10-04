package aichat

import (
	"context"
	"fmt"
	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/rpc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("LLM list tools", func() {
	It("returns the same bounded Markdown as the CLI renderer", func() {
		rows := make([]map[string]string, 26)
		for i := range rows {
			rows[i] = map[string]string{"name": fmt.Sprintf("item-%03d", i)}
		}
		page := entity.NewPagedResult(rows, 100, 0, 26)
		op := rpc.RPCOperation{Name: "records_list", Command: rpc.NewCobraExecutableCommand(&cobra.Command{Use: "records-list", RunE: func(*cobra.Command, []string) error { return fmt.Errorf("use structured execution") }}), Clicky: &entity.ClickyOperationMeta{Verb: "list"}, ContextDataFunc: func(context.Context, map[string]string, []string) (any, error) { return page, nil }}
		service := &rpc.RPCService{Operations: []rpc.RPCOperation{op}}
		provider := &CobraToolProvider{service: service, executor: rpc.NewCommandExecutor(service, &rpc.ExecutorConfig{Enabled: true})}
		result, err := provider.handlerFor(&op, context.Background())(context.Background(), map[string]any{})
		Expect(err).NotTo(HaveOccurred())
		expected, err := formatters.NewFormatManager().FormatWithOptions(formatters.FormatOptions{Format: "llm"}, page)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(expected))
		set, err := provider.ToolSet(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(set.Catalog).To(HaveLen(1))
		Expect(set.Catalog[0].OutputSchema).To(HaveKeyWithValue("type", "string"))
	})
})
