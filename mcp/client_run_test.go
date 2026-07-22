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
