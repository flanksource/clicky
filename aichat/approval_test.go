package aichat

import (
	"context"
	"testing"
)

func TestRequireApprovalForEmptyIsNil(t *testing.T) {
	if requireApprovalFor(nil) != nil {
		t.Error("requireApprovalFor(nil) should be nil (auto-approve everything)")
	}
}

func TestRequireApprovalForGatesNamedTools(t *testing.T) {
	pred := requireApprovalFor([]string{"stack_restart", "stack_delete"})
	if pred == nil {
		t.Fatal("expected a predicate for a non-empty list")
	}
	if !pred(ToolInfo{Name: "stack_restart"}, nil) {
		t.Error("stack_restart should require approval")
	}
	if pred(ToolInfo{Name: "stack_list"}, nil) {
		t.Error("stack_list should not require approval")
	}
}

func TestToolPreferencesOverrideDefaultApproval(t *testing.T) {
	defaultGate := func(tool ToolInfo, _ any) bool {
		return tool.Name != "stack_list"
	}
	ctx := withToolRuntime(context.Background(), toolRuntimeConfig{
		preferences: ToolPreferences{
			"stack_list":   ToolModeAsk,
			"stack_delete": ToolModeEnabled,
		},
		defaultApproval: defaultGate,
	})

	if !shouldRequireApproval(ctx, nil, ToolInfo{Name: "stack_list"}, nil) {
		t.Error("ask preference should force approval")
	}
	if shouldRequireApproval(ctx, nil, ToolInfo{Name: "stack_delete"}, nil) {
		t.Error("enabled preference should bypass default approval")
	}
	if !shouldRequireApproval(ctx, nil, ToolInfo{Name: "other_delete"}, nil) {
		t.Error("missing preference should use default approval")
	}
}

func TestToolsForRequestDropsDisabledTools(t *testing.T) {
	tools := []registeredTool{
		{ref: namedTool("stack_list"), info: ToolInfo{Name: "stack_list"}},
		{ref: namedTool("stack_delete"), info: ToolInfo{Name: "stack_delete"}},
	}
	refs := toolsForRequest(tools, ToolPreferences{"stack_delete": ToolModeDisabled})
	if len(refs) != 1 || refs[0].Name() != "stack_list" {
		t.Fatalf("refs = %+v, want only stack_list", refs)
	}
}

func TestCatalogInfoMarksConfiguredProviders(t *testing.T) {
	info := CatalogInfo([]Provider{ProviderAnthropic})
	if len(info) != len(catalog) {
		t.Fatalf("CatalogInfo len = %d, want %d", len(info), len(catalog))
	}
	var sawConfigured, sawUnconfigured bool
	for _, m := range info {
		if m.Label == "" {
			t.Errorf("model %q has no label", m.ID)
		}
		if m.Provider == string(ProviderAnthropic) {
			if !m.Configured {
				t.Errorf("%q should be configured", m.ID)
			}
			sawConfigured = true
		} else if m.Configured {
			t.Errorf("%q should not be configured (provider absent)", m.ID)
		} else {
			sawUnconfigured = true
		}
	}
	if !sawConfigured || !sawUnconfigured {
		t.Error("expected a mix of configured and unconfigured models")
	}
}

func TestMemThreadStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := NewMemThreadStore()
	th, err := store.Create(ctx, "Demo")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if th.ID == "" || th.Title != "Demo" {
		t.Fatalf("created thread = %+v", th)
	}
	if err := store.AppendMessage(ctx, th.ID, UIMessage{Role: "user", Parts: []UIPart{{Type: "text", Text: "hi"}}}); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	got, err := store.Get(ctx, th.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Messages) != 1 || got.Messages[0].Parts[0].Text != "hi" {
		t.Errorf("thread messages = %+v, want one 'hi'", got.Messages)
	}
	if _, err := store.Get(ctx, "missing"); err == nil {
		t.Error("Get(missing) should fail loud")
	}
	if err := store.Delete(ctx, th.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(ctx, th.ID); err == nil {
		t.Error("Get(deleted) should fail loud")
	}
}

type namedTool string

func (t namedTool) Name() string { return string(t) }
