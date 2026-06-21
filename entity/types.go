package entity

import (
	"context"

	"github.com/flanksource/clicky/api"
)

// Help is implemented by options structs that supply their own long-form command
// help. The generated command sets cmd.Long from Help().ANSI().
type Help interface {
	Help() api.Textable
}

// ExecuteOptions controls how an ExecutableCommand runs one invocation. It is the
// transport-neutral subset of what the RPC executor and MCP server previously
// poked directly onto a *cobra.Command.
type ExecuteOptions struct {
	// Args are the positional arguments.
	Args []string
	// Flags maps flag name -> string value. A flag absent on the command is
	// silently skipped (path params and other non-flag keys may appear here);
	// a present flag whose value fails to parse returns an error.
	Flags map[string]string
	// FormatOverrides are best-effort flag values applied before execution
	// (e.g. {"format":"markdown","no-color":"true"}). A flag absent on the
	// command is silently skipped. Used by the MCP server's format forcing.
	FormatOverrides map[string]string
}

// ExecutableCommand is the transport-neutral handle an RPCOperation holds onto in
// place of a concrete *cobra.Command. It encapsulates everything the execution
// path (RPC executor, MCP server) and the tool-surfacing layers (aichat) need, so
// those packages no longer reference cobra directly.
//
// The cobra-specific machinery — the global os.Stdout/os.Stderr pipe swap, flag
// copying, state reset — lives entirely inside the CobraExecutableCommand
// implementation (rpc package).
type ExecutableCommand interface {
	// Execute runs the command with the given options, capturing everything
	// written to os.Stdout / os.Stderr (including direct writes, not just the
	// cobra writers). It is safe for concurrent callers: implementations that
	// swap process-global file descriptors serialize internally.
	Execute(ctx context.Context, opts ExecuteOptions) (stdout, stderr string, err error)

	// Path is the full space-joined command path ("user create").
	Path() string
	// Name is the leaf command name ("create").
	Name() string
	// RootName is the top-level application command name ("xero-cli"), used for
	// the MCP tool title.
	RootName() string

	// Runnable reports whether the command has a Run/RunE (not a pure group).
	Runnable() bool
	// Hidden reports whether the command is hidden from help/listing.
	Hidden() bool

	// IsBoolFlag reports whether the named flag is a boolean flag. Used by the
	// CLI-string reconstruction to render "--flag" vs "--flag=value".
	IsBoolFlag(name string) bool
}
