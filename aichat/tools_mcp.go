package aichat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	clickymcp "github.com/flanksource/clicky/mcp"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
)

// MCPServer configures one external MCP server the chat backend consumes as a
// tool source. Exactly one transport must be set.
type MCPServer struct {
	Name           string
	Command        string            // stdio: executable
	Args           []string          // stdio: args
	Env            []string          // stdio: KEY=VALUE entries
	URL            string            // remote: base URL (SSE / streamable HTTP)
	Headers        map[string]string // remote: headers
	StreamableHTTP bool              // remote: use streamable HTTP instead of SSE
}

// MCPRegisteredTools discovers external tools through clicky's MCP client and
// exposes them as dynamic Genkit tools. Each invocation gets its own connection
// so startup discovery does not leave subprocesses or HTTP sessions running.
func MCPRegisteredTools(ctx context.Context, _ *genkit.Genkit, servers []MCPServer) ([]registeredTool, error) {
	if len(servers) == 0 {
		return nil, nil
	}

	var registered []registeredTool
	seen := map[string]bool{}
	for _, server := range servers {
		serverName := mcpServerName(server.Name)
		cfg, err := mcpClientConfig(server)
		if err != nil {
			return nil, err
		}
		session, err := clickymcp.Dial(ctx, serverName, cfg)
		if err != nil {
			return nil, fmt.Errorf("connect to MCP server %q: %w", serverName, err)
		}
		tools, fetchErr := clickymcp.FetchCatalog(ctx, cfg, session)
		closeErr := session.Close()
		if fetchErr != nil {
			return nil, fmt.Errorf("discover MCP server %q tools: %w", serverName, fetchErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close MCP server %q discovery session: %w", serverName, closeErr)
		}

		for _, tool := range tools {
			entry, err := newMCPRegisteredTool(serverName, cfg, tool)
			if err != nil {
				return nil, err
			}
			if seen[entry.ref.Name()] {
				return nil, fmt.Errorf("duplicate MCP tool %q", entry.ref.Name())
			}
			seen[entry.ref.Name()] = true
			registered = append(registered, entry)
		}
	}
	return registered, nil
}

// MCPTools is the compatibility helper for callers that only need ToolRefs.
func MCPTools(ctx context.Context, g *genkit.Genkit, servers []MCPServer) ([]ai.ToolRef, error) {
	tools, err := MCPRegisteredTools(ctx, g, servers)
	if err != nil {
		return nil, err
	}
	return toolRefs(tools), nil
}

func mcpServerName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "unnamed"
	}
	return name
}

func mcpClientConfig(server MCPServer) (clickymcp.ServerConfig, error) {
	hasCommand := strings.TrimSpace(server.Command) != ""
	hasURL := strings.TrimSpace(server.URL) != ""
	if hasCommand == hasURL {
		return clickymcp.ServerConfig{}, fmt.Errorf("MCP server %q must configure exactly one of Command or URL", mcpServerName(server.Name))
	}

	cfg := clickymcp.ServerConfig{
		Command: server.Command,
		Args:    append([]string(nil), server.Args...),
		URL:     server.URL,
		Headers: cloneStringMap(server.Headers),
	}
	if hasCommand {
		cfg.Type = "stdio"
		environment, err := mcpEnvironment(server.Name, server.Env)
		if err != nil {
			return clickymcp.ServerConfig{}, err
		}
		cfg.Env = environment
		return cfg, nil
	}
	if len(server.Env) > 0 {
		return clickymcp.ServerConfig{}, fmt.Errorf("MCP server %q environment is only valid for stdio", mcpServerName(server.Name))
	}
	if server.StreamableHTTP {
		cfg.Type = "http"
	} else {
		cfg.Type = "sse"
	}
	return cfg, nil
}

func mcpEnvironment(serverName string, entries []string) (map[string]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	environment := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, ok := strings.Cut(entry, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("MCP server %q has invalid environment entry %q (want KEY=VALUE)", mcpServerName(serverName), entry)
		}
		environment[key] = value
	}
	return environment, nil
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func newMCPRegisteredTool(serverName string, cfg clickymcp.ServerConfig, tool clickymcp.CachedTool) (registeredTool, error) {
	inputSchema, err := mcpInputSchema(tool)
	if err != nil {
		return registeredTool{}, fmt.Errorf("decode MCP tool %q schema: %w", tool.Name, err)
	}
	name := serverName + "_" + tool.Name
	ref := ai.NewTool[any, any](name, tool.Description,
		func(toolCtx *ai.ToolContext, input any) (any, error) {
			arguments, err := mcpToolArguments(tool, input)
			if err != nil {
				return nil, err
			}
			return callMCPTool(toolCtx.Context, serverName, cfg, tool.Name, arguments)
		},
		ai.WithInputSchema(inputSchema),
	)
	info := ToolInfo{Name: name, OperationName: tool.Name}
	catalog := catalogEntryFromToolRef("mcp", ref, info)
	catalog.Server = serverName
	return registeredTool{ref: ref, info: info, catalog: &catalog}, nil
}

func mcpInputSchema(tool clickymcp.CachedTool) (map[string]any, error) {
	if len(tool.InputSchema) == 0 || string(tool.InputSchema) == "null" {
		return objectSchema(nil), nil
	}
	var schema map[string]any
	if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
		return nil, err
	}
	return objectSchema(schema), nil
}

func mcpToolArguments(tool clickymcp.CachedTool, input any) (map[string]any, error) {
	arguments := map[string]any{}
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("MCP tool %q arguments are not JSON-serializable: %w", tool.Name, err)
		}
		if err := json.Unmarshal(encoded, &arguments); err != nil {
			return nil, fmt.Errorf("MCP tool %q arguments must be an object: %w", tool.Name, err)
		}
	}

	var schema struct {
		Required []string `json:"required"`
	}
	if len(tool.InputSchema) > 0 && string(tool.InputSchema) != "null" {
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			return nil, fmt.Errorf("decode MCP tool %q requirements: %w", tool.Name, err)
		}
	}
	for _, required := range schema.Required {
		if _, exists := arguments[required]; !exists {
			return nil, fmt.Errorf("required field %q missing for tool %q", required, tool.Name)
		}
	}
	return arguments, nil
}

func callMCPTool(ctx context.Context, serverName string, cfg clickymcp.ServerConfig, toolName string, arguments map[string]any) (*mcpsdk.CallToolResult, error) {
	session, err := clickymcp.Dial(ctx, serverName, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to MCP server %q: %w", serverName, err)
	}
	defer session.Close()

	request := mcpsdk.CallToolRequest{}
	request.Params.Name = toolName
	request.Params.Arguments = arguments
	result, err := session.Caller.CallTool(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("call MCP tool %q on server %q: %w", toolName, serverName, err)
	}
	return result, nil
}
