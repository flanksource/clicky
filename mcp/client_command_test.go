package mcp

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestRenderClientListEscapesMarkdownCells(t *testing.T) {
	records := []clientListRecord{{
		Name: "demo", Transport: "stdio", Endpoint: "echo a|b\nnext", ToolCount: 1, Cache: "fresh",
		Tools: []CachedTool{{Name: "read|file", Description: "first line\r\nsecond | line"}},
	}}
	var output bytes.Buffer
	if err := renderClientList(&output, records, "markdown", true); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`| demo | stdio | echo a\|b<br>next | 1 | fresh |`,
		`| ↳ read\|file | | first line<br>second \| line | | |`,
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("markdown output missing %q:\n%s", want, output.String())
		}
	}
}

func TestAddOAuthNoVerifyPersistsConfiguration(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	output, err := executeMCPCommand(
		"add", "private", "https://example.com/mcp",
		"--oauth-client-id", "client-1",
		"--oauth-client-name", "clicky test",
		"--oauth-scope", "mcp.read mcp.write",
		"--oauth-redirect-uri", "http://127.0.0.1:7777/callback",
		"--no-verify",
	)
	if err != nil {
		t.Fatalf("add: %v\n%s", err, output)
	}
	cfg, ok, err := NewServerRegistry("testapp").Get("private")
	if err != nil || !ok {
		t.Fatalf("Get() = %#v, %v, %v", cfg, ok, err)
	}
	if cfg.OAuth == nil || cfg.OAuth.ClientID != "client-1" || cfg.OAuth.ClientName != "clicky test" {
		t.Fatalf("OAuth config = %#v", cfg.OAuth)
	}
	if !reflect.DeepEqual(cfg.OAuth.Scopes, []string{"mcp.read", "mcp.write"}) {
		t.Fatalf("OAuth scopes = %v", cfg.OAuth.Scopes)
	}
}

func TestAddNoBrowserRequiresOAuth(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := executeMCPCommand("add", "public", "https://example.com/mcp", "--no-browser", "--no-verify")
	if err == nil || !strings.Contains(err.Error(), "requires --oauth") {
		t.Fatalf("error = %v", err)
	}
}

func TestAddRejectsInvalidNameBeforeOAuthStorageBinding(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	_, err := executeMCPCommand("add", "../config", "https://example.com/mcp", "--oauth", "--no-verify")
	if err == nil || !strings.Contains(err.Error(), "invalid server name") {
		t.Fatalf("error = %v", err)
	}
	registry := NewServerRegistry("testapp")
	if _, statErr := os.Stat(registry.configPath()); !os.IsNotExist(statErr) {
		t.Fatalf("invalid name created registry state: %v", statErr)
	}
}

func TestAddRejectsLiteralOAuthSecret(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := executeMCPCommand(
		"add", "private", "https://example.com/mcp", "--oauth-client-id", "client", "--oauth-client-secret", "literal", "--no-verify",
	)
	if err == nil || !strings.Contains(err.Error(), "env:NAME") {
		t.Fatalf("error = %v", err)
	}
}
