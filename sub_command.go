package clicky

import (
	"strings"
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
// all entity parents have been created. If no command at parentName exists
// after entity generation, a thin parent command is created to host cmd.
//
// parentName may be a bare command name ("policy") or a slash-delimited path
// ("billing/policy"). A path is resolved one segment at a time beneath root;
// missing intermediates are created as thin grouping commands, and existing
// nodes (including entity-generated parents) are reused.
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
// parentName accepts the same bare-name-or-slash-path form as RegisterSubCommand.
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
		parent := findOrCreateAtPath(root, p.parentName)
		if p.build != nil {
			p.build(parent)
		} else {
			if GetCommandOpenAPIMeta(parent) != nil {
				actionName := p.cmd.Name()
				if actionName == "" {
					actionName = p.cmd.Use
				}
				annotateEntityOperationCommand(p.cmd, parent, "action", "", "collection", actionName, "", false, false, false)
			}
			parent.AddCommand(p.cmd)
		}
	}
	pendingSubCommands = nil
}

// findOrCreateAtPath resolves a slash-delimited path beneath root, creating
// any missing nodes via findOrCreateChild. A path with no separator behaves
// exactly like findOrCreateChild(root, path). Empty segments (leading/trailing
// or doubled slashes) are skipped.
func findOrCreateAtPath(root *cobra.Command, path string) *cobra.Command {
	cur := root
	for _, segment := range strings.Split(path, "/") {
		if segment == "" {
			continue
		}
		cur = findOrCreateChild(cur, segment)
	}
	return cur
}
