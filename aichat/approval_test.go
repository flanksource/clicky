package aichat

import (
	"context"
	"testing"

	capai "github.com/flanksource/captain/pkg/ai"
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

func TestToolModeNormalizesCanonicalAndLegacyLabels(t *testing.T) {
	tests := map[ToolMode]ToolMode{
		"enabled":  ToolModeOn,
		"disabled": ToolModeOff,
		"on":       ToolModeOn,
		"off":      ToolModeOff,
		"ask":      ToolModeAsk,
		"auto":     ToolModeAuto,
	}
	for input, want := range tests {
		got, ok := normalizeToolMode(input)
		if !ok || got != want {
			t.Fatalf("normalizeToolMode(%q) = (%q,%v), want (%q,true)", input, got, ok, want)
		}
	}
}

func TestDefaultPermissionModes(t *testing.T) {
	defaultGate := func(ToolInfo, any) bool { return true }
	ctx := withToolRuntime(context.Background(), toolRuntimeConfig{defaultApproval: defaultGate})

	if shouldRequireApproval(ctx, nil, ToolInfo{Name: "auto_run", DefaultPermission: ToolModeOn}, nil) {
		t.Error("default permission on should bypass approval")
	}
	if !shouldRequireApproval(ctx, nil, ToolInfo{Name: "manual", DefaultPermission: ToolModeAsk}, nil) {
		t.Error("default permission ask should require approval")
	}
	if shouldRequireApproval(ctx, nil, ToolInfo{Name: "hidden", DefaultPermission: ToolModeOff}, nil) {
		t.Error("default permission off should not require approval")
	}
	if !shouldRequireApproval(ctx, nil, ToolInfo{Name: "policy", DefaultPermission: ToolModeAuto}, nil) {
		t.Error("default permission auto should defer to the runtime default approval policy")
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

func TestToolsForRequestDropsDefaultOffTools(t *testing.T) {
	tools := []registeredTool{
		{ref: namedTool("visible"), info: ToolInfo{Name: "visible", DefaultPermission: ToolModeAuto}},
		{ref: namedTool("hidden"), info: ToolInfo{Name: "hidden", DefaultPermission: ToolModeOff}},
	}
	refs := toolsForRequest(tools, nil)
	if len(refs) != 1 || refs[0].Name() != "visible" {
		t.Fatalf("refs = %+v, want only visible", refs)
	}
}

func TestCatalogInfoMarksConfiguredProviders(t *testing.T) {
	info := capai.CatalogInfo(providerStrings([]Provider{ProviderAnthropic}))
	if len(info) != len(Catalog()) {
		t.Fatalf("CatalogInfo len = %d, want %d", len(info), len(Catalog()))
	}
	var sawConfigured, sawUnconfigured bool
	for _, m := range info {
		if m.Label == "" {
			t.Errorf("model %q has no label", m.ID)
		}
		// Agent models are gated on local backend availability, not on the
		// registered Genkit providers passed here, so skip them for this check.
		if model, err := LookupModel(m.ID); err == nil && model.IsAgent() {
			continue
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
