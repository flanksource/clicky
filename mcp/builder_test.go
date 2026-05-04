package mcp

import (
	"testing"

	"github.com/flanksource/clicky/formatters"
)

func TestBuilder_Defaults(t *testing.T) {
	b := NewMcpServer()
	cfg := b.Config()
	if cfg == nil {
		t.Fatal("Config() returned nil")
	}
	if cfg.Tools.AutoExpose {
		t.Errorf("default AutoExpose should be false")
	}
}

func TestBuilder_WithExcludeIncludeAutoExpose(t *testing.T) {
	cfg := NewMcpServer().
		WithExclude("mcp", "ui serve").
		WithInclude("ai .*").
		AutoExpose().
		Config()

	if !cfg.Tools.AutoExpose {
		t.Errorf("AutoExpose() did not set the flag")
	}
	if got := cfg.Tools.Exclude; len(got) < 2 || got[len(got)-2] != "mcp" || got[len(got)-1] != "ui serve" {
		t.Errorf("unexpected Exclude: %v", got)
	}
	if got := cfg.Tools.Include; len(got) < 1 || got[len(got)-1] != "ai .*" {
		t.Errorf("unexpected Include: %v", got)
	}
}

func TestBuilder_IgnoreParams(t *testing.T) {
	cfg := NewMcpServer().
		IgnoreParams("*", "--addr", "--ui").
		IgnoreParams("ai *", "--cache").
		Config()

	if len(cfg.Tools.IgnoredParams) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(cfg.Tools.IgnoredParams))
	}
	if cfg.Tools.IgnoredParams[0].ToolGlob != "*" {
		t.Errorf("first rule glob = %q, want *", cfg.Tools.IgnoredParams[0].ToolGlob)
	}
	if got := cfg.Tools.IgnoredParams[0].Params; len(got) != 2 || got[0] != "--addr" {
		t.Errorf("first rule params = %v", got)
	}
	if cfg.Tools.IgnoredParams[1].ToolGlob != "ai *" {
		t.Errorf("second rule glob = %q", cfg.Tools.IgnoredParams[1].ToolGlob)
	}
}

func TestBuilder_IgnoreParams_Empty(t *testing.T) {
	cfg := NewMcpServer().
		IgnoreParams("", "--ignored"). // empty glob discarded
		IgnoreParams("*").             // no params discarded
		Config()
	if len(cfg.Tools.IgnoredParams) != 0 {
		t.Errorf("expected empty rule list, got %v", cfg.Tools.IgnoredParams)
	}
}

func TestBuilder_WithFormat(t *testing.T) {
	cfg := NewMcpServer().
		WithFormat(formatters.FormatOptions{Markdown: true, NoColor: true}).
		Config()
	if cfg.Tools.Format == nil {
		t.Fatal("Format not set")
	}
	if !cfg.Tools.Format.Markdown {
		t.Errorf("Markdown not set")
	}
	if !cfg.Tools.Format.NoColor {
		t.Errorf("NoColor not set")
	}
}

func TestBuilder_BuildRequiresRoot(t *testing.T) {
	_, err := NewMcpServer().Build()
	if err == nil {
		t.Fatal("expected error when Build() called without root cmd")
	}
}

func TestBuilder_Command(t *testing.T) {
	cmd := NewMcpServer().
		WithExclude("foo").
		Command()
	if cmd == nil {
		t.Fatal("Command() returned nil")
	}
	if cmd.Use != "mcp" {
		t.Errorf("expected Use=mcp, got %q", cmd.Use)
	}
}
