package aichat

import (
	"context"
	"testing"

	"github.com/firebase/genkit/go/genkit"
)

func TestDefineCustomToolsRegistersAndFilters(t *testing.T) {
	g := genkit.Init(context.Background())
	tools, err := DefineCustomTools(g, []ToolDefinition{{
		Name:        "xero_formula_patch",
		Description: "Return a formula replacement client action.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		Handler: func(context.Context, any) (any, error) {
			return map[string]any{"ok": true}, nil
		},
	}})
	if err != nil {
		t.Fatalf("DefineCustomTools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("tools = %d, want 1", len(tools))
	}
	if tools[0].info.Name != "xero_formula_patch" {
		t.Fatalf("tool name = %q", tools[0].info.Name)
	}
	if got := toolsForRequest(tools, ToolPreferences{"xero_formula_patch": ToolModeDisabled}); len(got) != 0 {
		t.Fatalf("disabled tool refs = %d, want 0", len(got))
	}
	if got := toolsForRequest(tools, ToolPreferences{"xero_formula_patch": ToolModeEnabled}); len(got) != 1 {
		t.Fatalf("enabled tool refs = %d, want 1", len(got))
	}
}

func TestCustomToolHonorsAskPreference(t *testing.T) {
	cfg := toolRuntimeConfig{
		preferences: ToolPreferences{"xero_formula_patch": ToolModeAsk},
	}
	if !shouldRequireApproval(withToolRuntime(context.Background(), cfg), nil, ToolInfo{Name: "xero_formula_patch"}, map[string]any{}) {
		t.Fatal("custom tool with ask preference should require approval")
	}
}

func TestDefineCustomToolsRejectsMissingHandler(t *testing.T) {
	g := genkit.Init(context.Background())
	_, err := DefineCustomTools(g, []ToolDefinition{{Name: "missing_handler"}})
	if err == nil {
		t.Fatal("DefineCustomTools succeeded with missing handler")
	}
}
