package mcp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestInstallShimIsOfflineExecutableAndIdempotent(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", t.TempDir())
	registry := NewServerRegistry("captain")
	if err := registry.Add("broken", ServerConfig{Type: "stdio", Command: "/does/not/exist"}); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	installed, err := InstallShim("captain", "broken", ShimOptions{
		BinDir: binDir, AllowTools: []string{"mcp__broken__read_*"}, DenyTools: []string{"mcp__broken__read_secret"},
	})
	if err != nil {
		t.Fatalf("InstallShim connected to an unreachable server or failed: %v", err)
	}
	info, err := os.Stat(installed)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	content, err := os.ReadFile(installed)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"mcp run 'broken'", "--allow-tool 'mcp__broken__read_*'", "--deny-tool 'mcp__broken__read_secret'", `-- "$@"`} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("shim missing %q:\n%s", want, content)
		}
	}
	second, err := InstallShim("captain", "broken", ShimOptions{
		BinDir: binDir, AllowTools: []string{"mcp__broken__read_*"}, DenyTools: []string{"mcp__broken__read_secret"},
	})
	if err != nil || second != installed {
		t.Fatalf("idempotent install = %q, %v", second, err)
	}
}

func TestInstallPromptShortcutsNamesAndFiltersServers(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	registry := NewServerRegistry("captain")
	for _, name := range []string{"github", "slack", "filesystem"} {
		if err := registry.Add(name, ServerConfig{Type: "stdio", Command: name}); err != nil {
			t.Fatal(err)
		}
	}
	binDir := t.TempDir()
	paths, err := installPromptShortcuts("captain", PromptRestrictions{
		Name: "review.prompt", Servers: []string{"github", "filesystem"},
		DisabledServers: []string{"filesystem"}, AllowTools: []string{"mcp__github__get_*"},
	}, binDir, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(binDir, "review-github")}
	if len(paths) != 1 || paths[0] != want[0] {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestInstallShimRefusesOverwrite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	registry := NewServerRegistry("captain")
	if err := registry.Add("demo", ServerConfig{Type: "stdio", Command: "demo"}); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	path := filepath.Join(binDir, "demo")
	if err := os.WriteFile(path, []byte("user file"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallShim("captain", "demo", ShimOptions{BinDir: binDir}); err == nil {
		t.Fatal("InstallShim overwrote an unrelated file")
	}
}

func TestInstallCommandRejectsFlagsFromOtherModes(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"client registration rejects bin dir", []string{"install", "--bin-dir", t.TempDir()}},
		{"server shortcut rejects global", []string{"install", "demo", "--global"}},
		{"prompt shortcut rejects client", []string{"install", "--prompt", "review.prompt", "--client", "codex"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := &cobra.Command{Use: "captain", SilenceUsage: true, SilenceErrors: true}
			root.AddCommand(newInstallCommandWithOptions(&CommandOptions{}, ClientOptions{
				ResolvePrompt: func(path string) (PromptRestrictions, error) {
					return PromptRestrictions{Name: "review"}, nil
				},
			}))
			var output bytes.Buffer
			root.SetOut(&output)
			root.SetErr(&output)
			root.SetArgs(tt.args)
			if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "does not apply") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
