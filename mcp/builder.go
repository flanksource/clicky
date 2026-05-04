package mcp

import (
	"github.com/flanksource/clicky/formatters"
	"github.com/spf13/cobra"
)

// Builder is a fluent helper for assembling an MCP server configuration.
// All state is held on an underlying *Config; methods append to slices or
// overwrite scalar fields, then return the receiver for chaining.
//
// Example:
//
//	mcp.NewMcpServer(rootCmd).
//	    WithExclude("mcp", "ui serve").
//	    IgnoreParams("*", "--addr", "--ui").
//	    WithFormat(formatters.FormatOptions{Markdown: true, NoColor: true}).
//	    Command()
type Builder struct {
	cfg     *Config
	rootCmd *cobra.Command
}

// NewMcpServer creates a Builder seeded with DefaultConfig(). Pass the
// host CLI's root cobra.Command if you want Build() to return a
// fully-initialised *MCPServer; otherwise Command() can be used to wire
// the `mcp` cobra subtree onto your CLI without immediate server start.
func NewMcpServer(rootCmd ...*cobra.Command) *Builder {
	b := &Builder{cfg: DefaultConfig()}
	if len(rootCmd) > 0 {
		b.rootCmd = rootCmd[0]
	}
	return b
}

// WithConfig replaces the underlying Config. Useful when callers want
// to start from a loaded JSON config and chain further overrides.
func (b *Builder) WithConfig(cfg *Config) *Builder {
	if cfg != nil {
		b.cfg = cfg
	}
	return b
}

// WithExclude appends regex patterns to ToolsConfig.Exclude. Patterns
// match against the cobra command path ("ui serve", "ai cache"), as
// applied by shouldExposeCommand.
func (b *Builder) WithExclude(patterns ...string) *Builder {
	b.cfg.Tools.Exclude = append(b.cfg.Tools.Exclude, patterns...)
	return b
}

// WithInclude appends regex patterns to ToolsConfig.Include. Has no
// effect when AutoExpose is true.
func (b *Builder) WithInclude(patterns ...string) *Builder {
	b.cfg.Tools.Include = append(b.cfg.Tools.Include, patterns...)
	return b
}

// AutoExpose flips ToolsConfig.AutoExpose to true so every cobra command
// (subject to Exclude rules) is registered as an MCP tool.
func (b *Builder) AutoExpose() *Builder {
	b.cfg.Tools.AutoExpose = true
	return b
}

// IgnoreParams strips the named parameters from MCP-facing tool schemas.
// toolGlob uses path.Match syntax: "*" matches every tool, "ai *"
// matches subcommands of ai, and a bare name matches one tool exactly.
// Param names may include the leading "--" or omit it.
//
// MCP-only — OpenAPI schemas generated from the same cobra tree are
// untouched.
func (b *Builder) IgnoreParams(toolGlob string, params ...string) *Builder {
	if toolGlob == "" || len(params) == 0 {
		return b
	}
	b.cfg.Tools.IgnoredParams = append(b.cfg.Tools.IgnoredParams, IgnoredParamRule{
		ToolGlob: toolGlob,
		Params:   params,
	})
	return b
}

// WithFormat sets the format/color overrides applied to every tool
// execution by injecting matching --format and --no-color flag values
// before the underlying cobra command runs. Tools without those flags
// are left untouched.
func (b *Builder) WithFormat(opts formatters.FormatOptions) *Builder {
	b.cfg.Tools.Format = &opts
	return b
}

// Config returns the underlying *Config. Mutations on the returned
// pointer are visible to subsequent Build/Command calls.
func (b *Builder) Config() *Config {
	return b.cfg
}

// Build constructs and initialises an *MCPServer using the configured
// values. Requires a rootCmd to have been supplied to NewMcpServer.
func (b *Builder) Build() (*MCPServer, error) {
	if b.rootCmd == nil {
		return nil, ErrBuilderNoRoot
	}
	srv := NewMCPServer(b.cfg, b.rootCmd)
	if err := srv.Initialize(); err != nil {
		return nil, err
	}
	return srv, nil
}

// Command returns the `mcp` cobra command group wired with the
// configured initial config. Add it to your CLI's root command tree.
func (b *Builder) Command() *cobra.Command {
	return NewCommandWithConfig(b.cfg)
}

// ErrBuilderNoRoot is returned by Build() when the Builder was created
// without a root cobra.Command.
var ErrBuilderNoRoot = builderError("Builder.Build() requires a rootCmd; pass it to NewMcpServer or use Command() to defer initialisation")

type builderError string

func (e builderError) Error() string { return string(e) }
