package aichat

import (
	"context"
	"fmt"
	"reflect"

	"github.com/flanksource/captain/pkg/api"
	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/rpc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("LLM list tools", func() {
	type record struct {
		Name string `json:"name"`
	}
	rows := make([]record, 26)
	for i := range rows {
		rows[i] = record{Name: fmt.Sprintf("item-%03d", i)}
	}
	page := entity.NewPagedResult(rows, 100, 0, 26)
	op := rpc.RPCOperation{
		Name:    "records_list",
		Command: rpc.NewCobraExecutableCommand(&cobra.Command{Use: "records-list", RunE: func(*cobra.Command, []string) error { return fmt.Errorf("use structured execution") }}),
		Clicky:  &entity.ClickyOperationMeta{Verb: "list"}, ResponseType: reflect.TypeOf(record{}), ResponsePaged: true,
		ContextDataFunc: func(context.Context, map[string]string, []string) (any, error) { return page, nil },
	}
	service := &rpc.RPCService{Operations: []rpc.RPCOperation{op}}
	provider := &CobraToolProvider{service: service, executor: rpc.NewCommandExecutor(service, &rpc.ExecutorConfig{Enabled: true})}

	It("gives the model the CLI's bounded Markdown and the UI the page it was rendered from", func() {
		result, err := provider.handlerFor(&op, context.Background())(context.Background(), map[string]any{})
		Expect(err).NotTo(HaveOccurred())
		expected, err := formatters.NewFormatManager().FormatWithOptions(formatters.FormatOptions{Format: "llm"}, page)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(api.StructuredToolResult{Content: expected, Structured: page}))
	})

	It("publishes the paged envelope the structured result follows as the output schema", func() {
		set, err := provider.ToolSet(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(set.Catalog).To(HaveLen(1))
		properties, ok := set.Catalog[0].OutputSchema["properties"].(map[string]any)
		Expect(ok).To(BeTrue(), "output schema: %#v", set.Catalog[0].OutputSchema)
		Expect(properties).To(HaveKey("page"))
		Expect(properties["data"]).To(HaveKeyWithValue("type", "array"))
	})
})
