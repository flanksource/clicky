package mcp

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ToolRegistry manages the mapping between cobra commands and MCP tools
type ToolRegistry struct {
	config *Config
	tools  map[string]*ToolDefinition
}

// ToolDefinition represents an MCP tool definition
type ToolDefinition struct {
	Name        string      `json:"name"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	InputSchema Schema      `json:"inputSchema"`
	OutputSchema *Schema    `json:"outputSchema,omitempty"`
	Command     *cobra.Command `json:"-"` // Internal reference
}

// Schema represents a JSON schema for tool input/output
type Schema struct {
	Type       string            `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string          `json:"required"`
}

// Property represents a JSON schema property
type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(config *Config) *ToolRegistry {
	return &ToolRegistry{
		config: config,
		tools:  make(map[string]*ToolDefinition),
	}
}

// RegisterCommand registers a cobra command as an MCP tool
func (r *ToolRegistry) RegisterCommand(cmd *cobra.Command) error {
	// Check if command should be exposed
	if !r.shouldExposeCommand(cmd) {
		return nil
	}
	
	// Generate tool definition
	tool, err := r.commandToTool(cmd)
	if err != nil {
		return fmt.Errorf("failed to convert command to tool: %w", err)
	}
	
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

// shouldExposeCommand determines if a command should be exposed as an MCP tool
func (r *ToolRegistry) shouldExposeCommand(cmd *cobra.Command) bool {
	cmdPath := getCommandPath(cmd)
	
	// Skip root command and commands without Run function
	if cmd.Parent() == nil || (cmd.Run == nil && cmd.RunE == nil) {
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

// commandToTool converts a cobra command to an MCP tool definition
func (r *ToolRegistry) commandToTool(cmd *cobra.Command) (*ToolDefinition, error) {
	cmdPath := getCommandPath(cmd)
	
	// Build input schema from flags
	schema := Schema{
		Type:       "object",
		Properties: make(map[string]Property),
		Required:   []string{},
	}
	
	// Add positional arguments
	if cmd.Args != nil {
		schema.Properties["args"] = Property{
			Type:        "array",
			Description: "Positional arguments for the command",
		}
	}
	
	// Process flags
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		
		prop := Property{
			Description: flag.Usage,
		}
		
		// Determine type based on flag type
		switch flag.Value.Type() {
		case "bool":
			prop.Type = "boolean"
			if flag.DefValue != "" {
				prop.Default = flag.DefValue == "true"
			}
		case "int", "int8", "int16", "int32", "int64":
			prop.Type = "integer"
			if flag.DefValue != "" {
				prop.Default = flag.DefValue
			}
		case "float32", "float64":
			prop.Type = "number"
			if flag.DefValue != "" {
				prop.Default = flag.DefValue
			}
		default:
			prop.Type = "string"
			if flag.DefValue != "" {
				prop.Default = flag.DefValue
			}
		}
		
		flagName := flag.Name
		schema.Properties[flagName] = prop
		
		// Mark required flags
		if err := cmd.MarkFlagRequired(flag.Name); err == nil {
			schema.Required = append(schema.Required, flagName)
		}
	})
	
	// Get description from config override or command
	description := cmd.Short
	if override, exists := r.config.Tools.Descriptions[cmdPath]; exists {
		description = override
	}
	
	// Get command name for title
	appName := "app"
	if root := getRootCommand(cmd); root != nil {
		appName = root.Name()
	}
	
	tool := &ToolDefinition{
		Name:        cmdPath,
		Title:       fmt.Sprintf("%s %s", appName, cmdPath),
		Description: description,
		InputSchema: schema,
		Command:     cmd,
	}
	
	return tool, nil
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