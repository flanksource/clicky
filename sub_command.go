package clicky

import (
	"sync"

	"github.com/spf13/cobra"
)

var (
	pendingSubCommands   []pendingSubCommand
	pendingSubCommandsMu sync.Mutex
)

type pendingSubCommand struct {
	parentName string
	cmd        *cobra.Command
	build      func(parent *cobra.Command)
}

// RegisterSubCommand defers attaching cmd as a child of a parent cobra command
// identified by parentName. The attachment is performed by GenerateCLI after
// all entity parents have been created. If no command named parentName exists
// after entity generation, a thin parent command is created to host cmd.
//
// Use this for non-entity cobra commands that should live under an
// entity-generated parent (or under an arbitrary grouping command).
func RegisterSubCommand(parentName string, cmd *cobra.Command) {
	pendingSubCommandsMu.Lock()
	defer pendingSubCommandsMu.Unlock()
	pendingSubCommands = append(pendingSubCommands, pendingSubCommand{
		parentName: parentName,
		cmd:        cmd,
	})
}

// RegisterSubCommandFn defers running build against a parent cobra command
// identified by parentName. Use this when the subcommand needs to be constructed
// lazily — e.g. AddNamedCommand which both builds and attaches in one call.
//
// Example:
//
//	clicky.RegisterSubCommandFn("correspondence", func(parent *cobra.Command) {
//	    clicky.AddNamedCommand("validate", parent, ValidateOptions{}, runValidate)
//	})
func RegisterSubCommandFn(parentName string, build func(parent *cobra.Command)) {
	pendingSubCommandsMu.Lock()
	defer pendingSubCommandsMu.Unlock()
	pendingSubCommands = append(pendingSubCommands, pendingSubCommand{
		parentName: parentName,
		build:      build,
	})
}

// flushPendingSubCommands attaches all deferred subcommands to their target
// parents under root. It is called by GenerateCLI.
func flushPendingSubCommands(root *cobra.Command) {
	pendingSubCommandsMu.Lock()
	defer pendingSubCommandsMu.Unlock()

	for _, p := range pendingSubCommands {
		parent := findOrCreateChild(root, p.parentName)
		if p.build != nil {
			p.build(parent)
		} else {
			if GetCommandOpenAPIMeta(parent) != nil {
				actionName := p.cmd.Name()
				if actionName == "" {
					actionName = p.cmd.Use
				}
				annotateEntityOperationCommand(p.cmd, parent, "action", "collection", actionName, "", false, false)
			}
			parent.AddCommand(p.cmd)
		}
	}
	pendingSubCommands = nil
}
