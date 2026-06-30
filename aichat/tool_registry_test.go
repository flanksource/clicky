package aichat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestHandleToolsServesCustomToolCatalog(t *testing.T) {
	s := NewServer(Options{
		CustomTools: []ToolDefinition{{
			Name:        "formula patch",
			Description: "Return a formula replacement client action.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"formula": map[string]any{"type": "string"}},
				"required":   []any{"formula"},
			},
			Group:   "Formula",
			Handler: func(context.Context, any) (any, error) { return map[string]any{"ok": true}, nil },
		}},
	})
	defer s.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/chat/tools", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var catalog ToolCatalog
	if err := json.Unmarshal(w.Body.Bytes(), &catalog); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if len(catalog.Tools) != 1 {
		t.Fatalf("tools = %d, want 1", len(catalog.Tools))
	}
	tool := catalog.Tools[0]
	if tool.Name != "formula_patch" || tool.Source != "custom" || tool.PreferenceKey != "Formula" {
		t.Fatalf("tool = %+v, want sanitized custom tool in Formula group", tool)
	}
	props := tool.InputSchema["properties"].(map[string]any)
	if _, ok := props["formula"]; !ok {
		t.Fatalf("input schema lost formula property: %v", tool.InputSchema)
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

func TestServerToolFilterHidesCustomTools(t *testing.T) {
	g := genkit.Init(context.Background())
	s := NewServer(Options{
		CustomTools: []ToolDefinition{
			{
				Name:    "visible",
				Group:   "Accounting Read",
				Handler: func(context.Context, any) (any, error) { return map[string]any{"ok": true}, nil },
			},
			{
				Name:    "hidden",
				Group:   "Disabled",
				Handler: func(context.Context, any) (any, error) { return map[string]any{"ok": true}, nil },
			},
		},
		ToolFilter: func(tool ToolInfo) bool {
			return tool.Group != "Disabled"
		},
	})

	tools, err := s.buildTools(context.Background(), g)
	if err != nil {
		t.Fatalf("buildTools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("tools = %d, want 1", len(tools))
	}
	if tools[0].info.Name != "visible" || tools[0].info.Group != "Accounting Read" {
		t.Fatalf("tool = %+v, want visible Accounting Read", tools[0].info)
	}
}
