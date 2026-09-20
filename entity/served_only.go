// The counterpart to local-only: operations that belong to the running server
// and would answer misleadingly if offered as commands in another process.
package entity

import (
	"fmt"

	"github.com/spf13/cobra"
)

// MarkServedOnly keeps a command off the CLI while leaving it on the generated
// HTTP and MCP surfaces.
//
// It is for operations over state that lives inside the running server: the
// monitors it is tracking, the sessions it is holding, the runs it has in
// flight. Such an operation is a perfectly good route, but as a command it runs
// in a different process and answers about that one — reporting no monitors
// when the server has ten. An empty answer that reads as a real one is worse
// than no command at all.
//
// This is the inverse of MarkLocalOnly, which keeps a command on the CLI and
// off the routes. An operation backed by a database, a cluster or a filesystem
// needs neither: it answers the same either way.
//
// It marks the whole subtree: marking a parent covers every subcommand under it.
func MarkServedOnly(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[annotationClickyServedOnly] = "true"
	cmd.Hidden = true
}

// IsServedOnly reports whether a command, or any command it is nested under, is
// marked served-only.
func IsServedOnly(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Annotations != nil && parseAnnotationBool(c.Annotations[annotationClickyServedOnly]) {
			return true
		}
	}
	return false
}

// RefuseServedOnlyCommands makes every served-only command in the tree explain
// itself instead of running.
//
// Hiding a command keeps it out of help but not out of reach; someone who knows
// the name can still type it. Call this once after the command tree is built —
// the same point a host application does its other whole-tree passes — so the
// answer is a reason rather than an empty result.
func RefuseServedOnlyCommands(root *cobra.Command) {
	if root == nil {
		return
	}
	for _, child := range root.Commands() {
		RefuseServedOnlyCommands(child)
	}
	if root.RunE == nil && root.Run == nil {
		return
	}
	if !IsServedOnly(root) {
		return
	}
	path := root.CommandPath()
	root.Run = nil
	root.RunE = func(*cobra.Command, []string) error {
		return fmt.Errorf("%s is served by the running server, not by this process: "+
			"it reports state held inside `serve`, so running it here would answer about nothing. "+
			"Call it over the API instead", path)
	}
}
