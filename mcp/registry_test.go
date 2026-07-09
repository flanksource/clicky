package mcp

import (
	"testing"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/rpc"
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

func TestNewMcpToolEmitsAnnotationsAndClickyMeta(t *testing.T) {
	readOnly := true
	destructive := false
	openWorld := false
	strict := true
	tool := NewMcpTool(&rpc.RPCOperation{
		Name:        "invoice list",
		Description: "List invoices",
		Method:      "GET",
		Schema: rpc.Schema{
			Type:       "object",
			Properties: map[string]rpc.Property{"status": {Type: "string"}},
		},
		ToolHints: entity.MCPToolHints{
			Title:             "Invoice list",
			ReadOnlyHint:      &readOnly,
			DestructiveHint:   &destructive,
			OpenWorldHint:     &openWorld,
			Icon:              "receipt",
			Group:             "billing",
			Parent:            "accounting",
			DefaultPermission: entity.ToolPermissionAsk,
			Strict:            &strict,
		},
	})

	if tool.Annotations == nil {
		t.Fatal("Annotations = nil, want MCP annotations")
	}
	if tool.Annotations.Title != "Invoice list" {
		t.Fatalf("annotation title = %q, want Invoice list", tool.Annotations.Title)
	}
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
		t.Fatalf("readOnlyHint = %v, want true", tool.Annotations.ReadOnlyHint)
	}
	if tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
		t.Fatalf("destructiveHint = %v, want false", tool.Annotations.DestructiveHint)
	}
	if tool.Annotations.IdempotentHint == nil || !*tool.Annotations.IdempotentHint {
		t.Fatalf("idempotentHint = %v, want inferred true for GET", tool.Annotations.IdempotentHint)
	}
	if tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
		t.Fatalf("openWorldHint = %v, want false", tool.Annotations.OpenWorldHint)
	}

	rawMeta, ok := tool.Meta[clickyToolMetaKey]
	if !ok {
		t.Fatalf("_meta missing %q: %+v", clickyToolMetaKey, tool.Meta)
	}
	meta, ok := rawMeta.(map[string]any)
	if !ok {
		t.Fatalf("_meta[%q] = %T, want map[string]any", clickyToolMetaKey, rawMeta)
	}
	if meta["icon"] != "receipt" || meta["group"] != "billing" || meta["parent"] != "accounting" {
		t.Fatalf("unexpected clicky _meta: %+v", meta)
	}
	if meta["defaultPermission"] != "ask" || meta["strict"] != true {
		t.Fatalf("permission/strict _meta = %+v, want ask/true", meta)
	}
}
