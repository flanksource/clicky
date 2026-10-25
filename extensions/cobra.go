package extensions

import (
	"github.com/flanksource/clicky/docs"
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

// DocsCommand adds the docs generation command group to the CLI
// Usage: extensions.CobraExtensions(rootCmd).DocsCommand()
func (c *CobraExtension) DocsCommand() *CobraExtension {
	c.rootCmd.AddCommand(docs.NewCommand())
	return c
}

// DocsCommandWithConfig adds the docs command group with custom configuration
func (c *CobraExtension) DocsCommandWithConfig(config *docs.DocsConfig) *CobraExtension {
	c.rootCmd.AddCommand(docs.NewCommandWithConfig(config))
	return c
}

// Interactive adds the persistent -i/--interactive flag: any runnable command
// then collects its arguments in a form (huh on a terminal, JsonSchemaForm on a
// dashboard prompt sink) before running. Call it after the command tree is
// complete — commands added later are not wired.
// Usage: extensions.CobraExtensions(rootCmd).Interactive()
func (c *CobraExtension) Interactive() *CobraExtension {
	rpc.InstallInteractive(c.rootCmd)
	return c
}

// All adds OpenAPI, MCP, and docs commands with default configuration, then
// wires --interactive across the tree.
// Usage: extensions.CobraExtensions(rootCmd).All()
func (c *CobraExtension) All() *CobraExtension {
	return c.OpenAPICommand().MCPCommand().DocsCommand().Interactive()
}

// ServeCommand adds OpenAPI serve functionality for Swagger UI documentation
// Usage: extensions.CobraExtensions(rootCmd).ServeCommand()
func (c *CobraExtension) ServeCommand() *CobraExtension {
	c.rootCmd.AddCommand(rpc.NewServeCommand())
	return c
}

// ServeCommandWithConfig adds OpenAPI serve functionality with custom configuration
func (c *CobraExtension) ServeCommandWithConfig(config *rpc.OpenAPIConfig) *CobraExtension {
	c.rootCmd.AddCommand(rpc.NewServeCommandWithConfig(config))
	return c
}

// AllWithConfig adds OpenAPI, MCP, and docs commands with custom configurations,
// then wires --interactive across the tree.
func (c *CobraExtension) AllWithConfig(openAPIConfig *rpc.OpenAPIConfig, mcpConfig *mcp.Config) *CobraExtension {
	return c.OpenAPICommandWithConfig(openAPIConfig).MCPCommandWithConfig(mcpConfig).DocsCommand().Interactive()
}
