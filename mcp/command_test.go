package mcp

import (
	"path/filepath"
	"testing"

	"github.com/flanksource/clicky/formatters"
)

func TestLoadCommandConfigMergesInitialHostDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp-config.json")

	initial := DefaultConfig()
	initial.Name = "gavel"
	initial.Description = "Gavel MCP"
	initial.Version = "test-version"
	initial.Security.RequireConfirmation = false
	initial.Security.TimeoutSeconds = 600
	initial.Tools.AutoExpose = true
	initial.Tools.Exclude = append(initial.Tools.Exclude, "^ui ")
	initial.Tools.Include = append(initial.Tools.Include, ".*")
	initial.Tools.IgnoredParams = []IgnoredParamRule{{ToolGlob: "*", Params: []string{"--format"}}}
	initial.Tools.Format = &formatters.FormatOptions{Markdown: true, NoColor: true}

	config, err := loadCommandConfig(path, initial)
	if err != nil {
		t.Fatalf("loadCommandConfig() error = %v", err)
	}

	if config.Name != "gavel" {
		t.Fatalf("Name = %q, want gavel", config.Name)
	}
	if config.Description != "Gavel MCP" {
		t.Fatalf("Description = %q", config.Description)
	}
	if config.Version != "test-version" {
		t.Fatalf("Version = %q", config.Version)
	}
	if config.Security.RequireConfirmation {
		t.Fatalf("RequireConfirmation should come from host initial config")
	}
	if config.Security.TimeoutSeconds != 600 {
		t.Fatalf("TimeoutSeconds = %d, want 600", config.Security.TimeoutSeconds)
	}
	if !config.Tools.AutoExpose {
		t.Fatalf("AutoExpose should be true")
	}
	if len(config.Tools.Exclude) == 0 || config.Tools.Exclude[len(config.Tools.Exclude)-1] != "^ui " {
		t.Fatalf("Exclude did not include host rule: %v", config.Tools.Exclude)
	}
	if countString(config.Tools.Include, ".*") != 1 {
		t.Fatalf("Include should be de-duplicated: %v", config.Tools.Include)
	}
	if countString(config.Tools.Exclude, "mcp") != 1 {
		t.Fatalf("Exclude should be de-duplicated: %v", config.Tools.Exclude)
	}
	if len(config.Tools.IgnoredParams) != 1 {
		t.Fatalf("IgnoredParams = %v", config.Tools.IgnoredParams)
	}
	if config.Tools.Format == nil || !config.Tools.Format.Markdown || !config.Tools.Format.NoColor {
		t.Fatalf("Format override not merged: %#v", config.Tools.Format)
	}
}

func countString(items []string, value string) int {
	count := 0
	for _, item := range items {
		if item == value {
			count++
		}
	}
	return count
}
