package extensions

import (
	"github.com/flanksource/clicky/mcp"
	"github.com/flanksource/clicky/rpc"
	"github.com/spf13/cobra"
)

// CobraExtension provides a fluent interface for adding clicky functionality to Cobra commands
type CobraExtension struct {
	rootCmd *cobra.Command
}

// CobraExtensions creates a new fluent extension builder for the given Cobra command
func CobraExtensions(rootCmd *cobra.Command) *CobraExtension {
	return &CobraExtension{rootCmd: rootCmd}
}

// OpenAPICommand adds OpenAPI generation and validation commands to the CLI
// Usage: extensions.CobraExtensions(rootCmd).OpenAPICommand()
func (c *CobraExtension) OpenAPICommand() *CobraExtension {
	c.rootCmd.AddCommand(rpc.NewOpenAPICommand())
	return c
}

// OpenAPICommandWithConfig adds OpenAPI commands with custom configuration
func (c *CobraExtension) OpenAPICommandWithConfig(config *rpc.OpenAPIConfig) *CobraExtension {
	c.rootCmd.AddCommand(rpc.NewOpenAPICommandWithConfig(config))
	return c
}

// MCPCommand adds MCP (Model Context Protocol) server functionality to the CLI
// Usage: extensions.CobraExtensions(rootCmd).MCPCommand()
func (c *CobraExtension) MCPCommand() *CobraExtension {
	c.rootCmd.AddCommand(mcp.NewCommand())
	return c
}

// MCPCommandWithConfig adds MCP commands with custom configuration
func (c *CobraExtension) MCPCommandWithConfig(config *mcp.Config) *CobraExtension {
	c.rootCmd.AddCommand(mcp.NewCommandWithConfig(config))
	return c
}

// All adds both OpenAPI and MCP commands with default configuration
// Usage: extensions.CobraExtensions(rootCmd).All()
func (c *CobraExtension) All() *CobraExtension {
	return c.OpenAPICommand().MCPCommand()
}

// ServeCommand adds OpenAPI serve functionality for Swagger UI documentation
// Usage: extensions.CobraExtensions(rootCmd).ServeCommand()
func (c *CobraExtension) ServeCommand() *CobraExtension {
	c.rootCmd.AddCommand(rpc.NewOpenAPICommand())
	return c
}

// ServeCommandWithConfig adds OpenAPI serve functionality with custom configuration
func (c *CobraExtension) ServeCommandWithConfig(config *rpc.OpenAPIConfig) *CobraExtension {
	c.rootCmd.AddCommand(rpc.NewOpenAPICommandWithConfig(config))
	return c
}

// AllWithConfig adds both OpenAPI and MCP commands with custom configurations
func (c *CobraExtension) AllWithConfig(openAPIConfig *rpc.OpenAPIConfig, mcpConfig *mcp.Config) *CobraExtension {
	return c.OpenAPICommandWithConfig(openAPIConfig).MCPCommandWithConfig(mcpConfig)
}