package mcp

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/flanksource/clicky/rpc"
	"github.com/spf13/cobra"
)

// ToolRegistry manages the mapping between cobra commands and MCP tools
type ToolRegistry struct {
	config       *Config
	tools        map[string]*ToolDefinition
	rpcConverter *rpc.Converter
}

// ToolDefinition represents an MCP tool definition
type ToolDefinition struct {
	Name         string         `json:"name"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	InputSchema  Schema         `json:"inputSchema"`
	OutputSchema *Schema        `json:"outputSchema,omitempty"`
	Command      *cobra.Command `json:"-"` // Internal reference
}

// Schema represents a JSON schema for tool input/output
type Schema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

// Property represents a JSON schema property
type Property struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Enum        []string    `json:"enum,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

// NewMcpTool creates an MCP ToolDefinition from a generic RPC operation
func NewMcpTool(rpcOp *rpc.RPCOperation) *ToolDefinition {
	// Convert RPC schema to MCP schema
	inputSchema := Schema{
		Type:       rpcOp.Schema.Type,
		Properties: make(map[string]Property),
		Required:   rpcOp.Schema.Required,
	}

	// Convert RPC properties to MCP properties
	for name, rpcProp := range rpcOp.Schema.Properties {
		inputSchema.Properties[name] = Property{
			Type:        rpcProp.Type,
			Description: rpcProp.Description,
			Enum:        rpcProp.Enum,
			Default:     rpcProp.Default,
		}
	}

	// Get application name for title
	appName := "app"
	if rpcOp.Command != nil {
		if root := getRootCommand(rpcOp.Command); root != nil {
			appName = root.Name()
		}
	}

	return &ToolDefinition{
		Name:        rpcOp.Name,
		Title:       fmt.Sprintf("%s %s", appName, rpcOp.Name),
		Description: rpcOp.Description,
		InputSchema: inputSchema,
		Command:     rpcOp.Command,
	}
}

// NewMcpToolWithConfig creates an MCP ToolDefinition from a generic RPC operation with config overrides
func (r *ToolRegistry) NewMcpToolWithConfig(rpcOp *rpc.RPCOperation) *ToolDefinition {
	tool := NewMcpTool(rpcOp)

	// Apply config-specific overrides
	if override, exists := r.config.Tools.Descriptions[rpcOp.Name]; exists {
		tool.Description = override
	}

	applyIgnoredParams(tool, r.config.Tools.IgnoredParams)

	return tool
}

// applyIgnoredParams strips matched parameter names from tool.InputSchema's
// Properties and Required, in place. Names in rules may carry a leading
// "--" (CLI form) or be bare; both are normalised to bare keys, matching
// how the RPC converter populates Schema.Properties.
func applyIgnoredParams(tool *ToolDefinition, rules []IgnoredParamRule) {
	if len(rules) == 0 || tool == nil {
		return
	}
	ignored := map[string]bool{}
	for _, rule := range rules {
		matched, err := path.Match(rule.ToolGlob, tool.Name)
		if err != nil || !matched {
			continue
		}
		for _, p := range rule.Params {
			ignored[strings.TrimPrefix(p, "--")] = true
		}
	}
	if len(ignored) == 0 {
		return
	}
	for name := range ignored {
		delete(tool.InputSchema.Properties, name)
	}
	if len(tool.InputSchema.Required) > 0 {
		filtered := tool.InputSchema.Required[:0]
		for _, r := range tool.InputSchema.Required {
			if !ignored[r] {
				filtered = append(filtered, r)
			}
		}
		tool.InputSchema.Required = filtered
	}
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(config *Config) *ToolRegistry {
	// Create RPC converter with default config
	rpcConfig := rpc.DefaultConfig()

	return &ToolRegistry{
		config:       config,
		tools:        make(map[string]*ToolDefinition),
		rpcConverter: rpc.NewConverter(rpcConfig),
	}
}

// RegisterCommand registers a cobra command as an MCP tool
func (r *ToolRegistry) RegisterCommand(cmd *cobra.Command) error {
	// Check if command should be exposed (MCP-specific filtering)
	if !r.shouldExposeCommand(cmd) {
		return nil
	}

	// Convert to RPC operation first
	rpcOp, err := r.rpcConverter.ConvertCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to convert command to RPC operation: %w", err)
	}

	// Create MCP tool from RPC operation with config overrides
	tool := r.NewMcpToolWithConfig(rpcOp)

	r.tools[tool.Name] = tool
	return nil
}

// RegisterCommandTree recursively registers a command and its subcommands
func (r *ToolRegistry) RegisterCommandTree(cmd *cobra.Command) error {
	// Register the command itself
	if err := r.RegisterCommand(cmd); err != nil {
		return err
	}

	// Register subcommands
	for _, subCmd := range cmd.Commands() {
		if err := r.RegisterCommandTree(subCmd); err != nil {
			return err
		}
	}

	return nil
}

// GetTools returns all registered tools
func (r *ToolRegistry) GetTools() map[string]*ToolDefinition {
	return r.tools
}

// GetTool returns a specific tool by name
func (r *ToolRegistry) GetTool(name string) (*ToolDefinition, bool) {
	tool, exists := r.tools[name]
	return tool, exists
}

var autoExcluded = map[string]bool{
	"help":       true,
	"completion": true,
	"version":    true,
	"mcp":        true,
}

// shouldExposeCommand determines if a command should be exposed as an MCP tool
func (r *ToolRegistry) shouldExposeCommand(cmd *cobra.Command) bool {
	cmdPath := getCommandPath(cmd)

	// Skip root command and commands without Run function
	if cmd.Parent() == nil || (cmd.Run == nil && cmd.RunE == nil) {
		return false
	}

	// Skip built-in commands
	topLevel := strings.SplitN(cmdPath, " ", 2)[0]
	if autoExcluded[topLevel] {
		return false
	}

	// Check blocked commands
	for _, blocked := range r.config.Tools.Exclude {
		if matched, _ := regexp.MatchString(blocked, cmdPath); matched {
			return false
		}
	}

	// If auto-expose is enabled, expose all non-blocked commands
	if r.config.Tools.AutoExpose {
		return true
	}

	// Check allowed commands
	for _, allowed := range r.config.Tools.Include {
		if matched, _ := regexp.MatchString(allowed, cmdPath); matched {
			return true
		}
	}

	return false
}

// getCommandPath returns the full command path (e.g., "status", "ai cache")
func getCommandPath(cmd *cobra.Command) string {
	if cmd.Parent() == nil {
		return cmd.Name()
	}

	parts := []string{}
	for c := cmd; c.Parent() != nil; c = c.Parent() {
		parts = append([]string{c.Name()}, parts...)
	}

	return strings.Join(parts, " ")
}

// getRootCommand returns the root command
func getRootCommand(cmd *cobra.Command) *cobra.Command {
	for cmd.Parent() != nil {
		cmd = cmd.Parent()
	}
	return cmd
}

// ListToolsResponse represents the MCP tools/list response
type ListToolsResponse struct {
	Tools []ToolDefinition `json:"tools"`
}

// ToListResponse converts the registry to an MCP tools/list response
func (r *ToolRegistry) ToListResponse() *ListToolsResponse {
	tools := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		// Create a copy without the internal Command field
		toolCopy := *tool
		toolCopy.Command = nil
		tools = append(tools, toolCopy)
	}

	return &ListToolsResponse{
		Tools: tools,
	}
}
