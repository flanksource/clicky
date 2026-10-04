package mcp

import (
	"context"
	"testing"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/rpc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

func TestLLMTools(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MCP LLM tools")
}

type llmRows struct{ done bool }

func (r *llmRows) Columns() []api.ColumnDef { return []api.ColumnDef{{Name: "name"}} }
func (r *llmRows) Next() bool {
	if r.done {
		return false
	}
	r.done = true
	return true
}
func (*llmRows) Row() map[string]any { return map[string]any{"name": "Example record"} }
func (*llmRows) Err() error          { return nil }
func (*llmRows) Close() error        { return nil }

var _ = Describe("LLM MCP paging", func() {
	It("defaults to native LLM paging even when only no-color is configured", func() {
		type scopeKey struct{}
		var scope any
		op := &rpc.RPCOperation{Name: "records_list", PagedFunc: func(ctx context.Context, req entity.PageRequest, flags map[string]string) (entity.PageResponse, error) {
			scope = ctx.Value(scopeKey{})
			Expect(req.Limit).To(Equal(api.LLMPageSize))
			Expect(req.Cursor).To(Equal("last-record"))
			Expect(flags).To(HaveKeyWithValue("account", "assets://example"))
			return entity.PageResponse{Rows: &llmRows{}, Pageable: true, HasMore: true, Next: "next-record"}, nil
		}}
		server := NewMCPServer(&Config{Tools: ToolsConfig{Format: &formatters.FormatOptions{NoColor: true}}}, &cobra.Command{Use: "example"})
		result, err := server.executeToolWithTaskManager(context.WithValue(context.Background(), scopeKey{}, "entity-x"), NewMcpTool(op), map[string]any{"cursor": "last-record", "limit": 100, "account": "assets://example"})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.IsError).To(BeFalse())
		Expect(scope).To(Equal("entity-x"))
		Expect(result.Content).To(HaveLen(1))
		Expect(result.Content[0].Text).To(ContainSubstring("Example record"))
		Expect(result.Content[0].Text).To(ContainSubstring(`--cursor "next-record" --limit 25`))
		Expect(result.Content[0].Text).To(ContainSubstring("current page"))
	})
})
