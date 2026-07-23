package aichat

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/firebase/genkit/go/ai"
	clickymcp "github.com/flanksource/clicky/mcp"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func TestMCPRegisteredToolsUsesClickyClient(t *testing.T) {
	server := mcpserver.NewMCPServer("aichat-test", "1.0.0", mcpserver.WithToolCapabilities(true))
	server.AddTool(mcpsdk.NewTool("echo",
		mcpsdk.WithDescription("Echo a message"),
		mcpsdk.WithString("message", mcpsdk.Required()),
	), func(_ context.Context, request mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		message, err := request.RequireString("message")
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("invalid message", err), nil
		}
		return mcpsdk.NewToolResultText(message), nil
	})
	handler := mcpserver.NewStreamableHTTPServer(
		server,
		mcpserver.WithStateLess(true),
		mcpserver.WithDisableStreaming(true),
	)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	registered, err := MCPRegisteredTools(context.Background(), nil, []MCPServer{{
		Name: "demo", URL: httpServer.URL, StreamableHTTP: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(registered) != 1 {
		t.Fatalf("registered tools = %d, want 1", len(registered))
	}
	if registered[0].ref.Name() != "demo_echo" {
		t.Fatalf("tool name = %q", registered[0].ref.Name())
	}
	if registered[0].catalog == nil || registered[0].catalog.Source != "mcp" || registered[0].catalog.Server != "demo" {
		t.Fatalf("catalog = %#v", registered[0].catalog)
	}

	tool, ok := registered[0].ref.(ai.Tool)
	if !ok {
		t.Fatalf("tool ref has type %T", registered[0].ref)
	}
	output, err := tool.RunRaw(context.Background(), map[string]any{"message": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := output.(map[string]any)
	if !ok {
		t.Fatalf("tool output = %#v (%T)", output, output)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("result content = %#v", result["content"])
	}
	text, ok := content[0].(map[string]any)
	if !ok || text["text"] != "hello" {
		t.Fatalf("result content = %#v", content[0])
	}
}

func TestMCPClientConfig(t *testing.T) {
	cfg, err := mcpClientConfig(MCPServer{
		Name: "local", Command: "server", Args: []string{"--stdio"},
		Env: []string{"TOKEN=secret", "VALUE=contains=equals"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Type != "stdio" || cfg.Command != "server" || !reflect.DeepEqual(cfg.Args, []string{"--stdio"}) {
		t.Fatalf("config = %#v", cfg)
	}
	if !reflect.DeepEqual(cfg.Env, map[string]string{"TOKEN": "secret", "VALUE": "contains=equals"}) {
		t.Fatalf("environment = %#v", cfg.Env)
	}

	for _, test := range []struct {
		name   string
		server MCPServer
		want   string
	}{
		{name: "missing transport", server: MCPServer{Name: "missing"}, want: "exactly one"},
		{name: "ambiguous transport", server: MCPServer{Name: "both", Command: "server", URL: "https://example.com"}, want: "exactly one"},
		{name: "invalid environment", server: MCPServer{Name: "local", Command: "server", Env: []string{"INVALID"}}, want: "KEY=VALUE"},
		{name: "remote environment", server: MCPServer{Name: "remote", URL: "https://example.com", Env: []string{"A=B"}}, want: "only valid for stdio"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := mcpClientConfig(test.server)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestMCPToolArgumentsValidatesRequiredFields(t *testing.T) {
	tool := clickymcp.CachedTool{
		Name: "echo",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"message":{"type":"string"}},
			"required":["message"]
		}`),
	}
	if _, err := mcpToolArguments(tool, map[string]any{}); err == nil || !strings.Contains(err.Error(), `required field "message"`) {
		t.Fatalf("error = %v", err)
	}
	arguments, err := mcpToolArguments(tool, map[string]any{"message": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if arguments["message"] != "hello" {
		t.Fatalf("arguments = %#v", arguments)
	}
}
