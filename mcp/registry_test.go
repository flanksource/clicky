package mcp

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestApplyIgnoredParams_StripsFlagsAndRequired(t *testing.T) {
	tool := &ToolDefinition{
		Name: "ai cache",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"addr":   {Type: "string"},
				"ui":     {Type: "boolean"},
				"prompt": {Type: "string"},
				"id":     {Type: "string"},
			},
			Required: []string{"id", "addr"},
		},
	}

	applyIgnoredParams(tool, []IgnoredParamRule{
		{ToolGlob: "*", Params: []string{"--addr", "ui"}},
	})

	if _, present := tool.InputSchema.Properties["addr"]; present {
		t.Errorf("addr should be stripped")
	}
	if _, present := tool.InputSchema.Properties["ui"]; present {
		t.Errorf("ui should be stripped")
	}
	if _, present := tool.InputSchema.Properties["prompt"]; !present {
		t.Errorf("prompt should be retained")
	}
	if got := tool.InputSchema.Required; len(got) != 1 || got[0] != "id" {
		t.Errorf("expected required=[id], got %v", got)
	}
}

func TestApplyIgnoredParams_Glob(t *testing.T) {
	mk := func(name string) *ToolDefinition {
		return &ToolDefinition{
			Name: name,
			InputSchema: Schema{
				Properties: map[string]Property{
					"addr":  {},
					"other": {},
				},
			},
		}
	}

	rules := []IgnoredParamRule{{ToolGlob: "ai *", Params: []string{"addr"}}}

	matched := mk("ai cache")
	applyIgnoredParams(matched, rules)
	if _, present := matched.InputSchema.Properties["addr"]; present {
		t.Errorf("ai cache: addr should be stripped")
	}

	unmatched := mk("status")
	applyIgnoredParams(unmatched, rules)
	if _, present := unmatched.InputSchema.Properties["addr"]; !present {
		t.Errorf("status: addr should be retained (glob doesn't match)")
	}
}

func TestApplyIgnoredParams_NoOpWhenEmpty(t *testing.T) {
	tool := &ToolDefinition{
		Name: "x",
		InputSchema: Schema{
			Properties: map[string]Property{"a": {}},
			Required:   []string{"a"},
		},
	}
	applyIgnoredParams(tool, nil)
	if len(tool.InputSchema.Properties) != 1 {
		t.Errorf("nil rules should be a no-op")
	}
}

// End-to-end: register a real cobra command tree and confirm IgnoredParams
// is applied during MCP tool registration but the OpenAPI/RPC converter
// (which mcp.Config never reaches) is unaffected.
func TestRegistry_RegisterCommand_AppliesIgnoredParams(t *testing.T) {
	root := &cobra.Command{Use: "app"}
	leaf := &cobra.Command{
		Use:   "ping",
		Short: "send a ping",
		Run:   func(cmd *cobra.Command, args []string) {},
	}
	leaf.Flags().String("addr", "", "server address")
	leaf.Flags().Bool("ui", false, "open UI")
	leaf.Flags().String("payload", "", "payload to send")
	root.AddCommand(leaf)

	cfg := DefaultConfig()
	cfg.Tools.AutoExpose = true
	cfg.Tools.IgnoredParams = []IgnoredParamRule{
		{ToolGlob: "*", Params: []string{"--addr", "--ui"}},
	}

	registry := NewToolRegistry(cfg)
	if err := registry.RegisterCommandTree(root); err != nil {
		t.Fatalf("RegisterCommandTree: %v", err)
	}

	tool, ok := registry.GetTool("ping")
	if !ok {
		t.Fatal("expected ping tool to be registered")
	}

	if _, present := tool.InputSchema.Properties["addr"]; present {
		t.Errorf("addr should be hidden from MCP schema")
	}
	if _, present := tool.InputSchema.Properties["ui"]; present {
		t.Errorf("ui should be hidden from MCP schema")
	}
	if _, present := tool.InputSchema.Properties["payload"]; !present {
		t.Errorf("payload should remain visible")
	}
}

func TestRegistry_NoDiscoverToolsBuiltin(t *testing.T) {
	r := NewToolRegistry(DefaultConfig())
	if _, ok := r.GetTool("discover-tools"); ok {
		t.Fatal("discover-tools must not be exposed as a regular MCP tool; it lives under `mcp tools`")
	}
}
