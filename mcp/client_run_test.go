package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
)

func TestRunToolCommandUsesCLIOnlyHelp(t *testing.T) {
	tool := CachedTool{
		Name: "ask_question",
		Description: `Ask a repository question.

Args:
    repoName: GitHub repository or list of repositories (max 10) in owner/repo format
    question: Description from the MCP tool comment`,
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"repoName":{"type":"string"},
				"question":{"type":"string","description":"The question to ask about the repository"}
			}
		}`),
	}
	cmd, err := newRunToolCommand(
		newServerRegistryAt("test", t.TempDir()),
		"demo",
		ServerConfig{Type: "http", URL: "https://example.com/mcp"},
		&CatalogCache{},
		tool,
	)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Long != "Ask a repository question." || strings.Contains(cmd.Long, "Args:") {
		t.Fatalf("Long = %q", cmd.Long)
	}
	if got := cmd.Flags().Lookup("repo-name").Usage; got != "GitHub repository or list of repositories (max 10) in owner/repo format" {
		t.Fatalf("--repo-name usage = %q", got)
	}
	if got := cmd.Flags().Lookup("question").Usage; got != "The question to ask about the repository" {
		t.Fatalf("--question usage = %q", got)
	}
}

func TestRunPolicyRejectsInvalidGlobs(t *testing.T) {
	for _, flag := range []string{"--allow-tool", "--deny-tool"} {
		t.Run(flag, func(t *testing.T) {
			_, _, err := parseRunPolicyArgs([]string{flag, "[", "--", "--help"})
			if err == nil || !strings.Contains(err.Error(), `invalid tool policy glob "["`) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestDecodedSizeDoesNotRequireDecodedPayload(t *testing.T) {
	tests := map[string]int{
		"":             0,
		"AQID":         3,
		"AQIDBA==":     4,
		"AQID\r\nBA==": 4,
		"!!!!":         0,
		"A===":         0,
	}
	for encoded, want := range tests {
		if got := decodedSize(encoded); got != want {
			t.Errorf("decodedSize(%q) = %d, want %d", encoded, got, want)
		}
	}
}

func TestRenderJSONErrorUsesStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	result := mcpsdk.NewToolResultError("failed")
	err := renderCallToolResult(&stdout, &stderr, "demo", result, true)
	if err == nil {
		t.Fatal("error result returned success")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `"isError": true`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
