package aichat

import (
	"context"
	"testing"
)

func groupedTools() []registeredTool {
	return []registeredTool{
		{info: ToolInfo{Name: "invoice_list", Group: "billing"}},
		{info: ToolInfo{Name: "invoice_get", Group: "billing"}},
		{info: ToolInfo{Name: "stack_list"}},
	}
}

func TestListToolEntriesCollapsesGroups(t *testing.T) {
	entries := ListToolEntries(groupedTools(), nil)

	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2 (one group + one ungrouped)", len(entries))
	}
	// Sorted by Key: "billing" < "stack_list".
	if entries[0].Key != "billing" || entries[0].Group != "billing" {
		t.Errorf("entries[0] = %+v, want billing group entry", entries[0])
	}
	if got := entries[0].Tools; len(got) != 2 || got[0] != "invoice_get" || got[1] != "invoice_list" {
		t.Errorf("group members = %v, want [invoice_get invoice_list] (sorted)", got)
	}
	if entries[1].Key != "stack_list" || entries[1].Group != "" {
		t.Errorf("entries[1] = %+v, want ungrouped stack_list", entries[1])
	}
}

func TestListToolEntriesAnnotatesMode(t *testing.T) {
	prefs := ToolPreferences{"billing": ToolModeDisabled}
	entries := ListToolEntries(groupedTools(), prefs)
	if entries[0].Key != "billing" || entries[0].Mode != ToolModeDisabled {
		t.Errorf("billing entry mode = %q, want disabled", entries[0].Mode)
	}
}

func TestEffectivePreferenceGroupGovernsMembers(t *testing.T) {
	prefs := ToolPreferences{"billing": ToolModeDisabled}
	for _, name := range []string{"invoice_list", "invoice_get"} {
		mode, ok := effectivePreference(prefs, ToolInfo{Name: name, Group: "billing"})
		if !ok || mode != ToolModeDisabled {
			t.Errorf("effectivePreference(%s) = (%q,%v), want (disabled,true)", name, mode, ok)
		}
	}

	refs := toolsForRequest(groupedTools(), prefs)
	if len(refs) != 1 {
		t.Errorf("toolsForRequest kept %d tools, want 1 (only ungrouped stack_list)", len(refs))
	}
}

func TestGroupAskForcesApprovalForMembers(t *testing.T) {
	ctx := withToolRuntime(context.Background(), toolRuntimeConfig{
		preferences: ToolPreferences{"billing": ToolModeAsk},
	})
	if !shouldRequireApproval(ctx, nil, ToolInfo{Name: "invoice_get", Group: "billing"}, nil) {
		t.Error("expected approval required for a member of an 'ask' group")
	}
}

func TestMemberKeyOverridesGroupKey(t *testing.T) {
	prefs := ToolPreferences{"billing": ToolModeEnabled, "invoice_get": ToolModeDisabled}
	mode, ok := effectivePreference(prefs, ToolInfo{Name: "invoice_get", Group: "billing"})
	if !ok || mode != ToolModeDisabled {
		t.Errorf("effectivePreference = (%q,%v), want (disabled,true) — member key must win", mode, ok)
	}
}

func TestUngroupedToolResolvesByName(t *testing.T) {
	prefs := ToolPreferences{"stack_delete": ToolModeDisabled}
	mode, ok := effectivePreference(prefs, ToolInfo{Name: "stack_delete"})
	if !ok || mode != ToolModeDisabled {
		t.Errorf("effectivePreference(ungrouped) = (%q,%v), want (disabled,true)", mode, ok)
	}
}
