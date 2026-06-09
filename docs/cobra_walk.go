package docs

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// walkCommands invokes fn for cmd and every descendant, depth-first.
func walkCommands(cmd *cobra.Command, fn func(*cobra.Command)) {
	fn(cmd)
	for _, sub := range cmd.Commands() {
		walkCommands(sub, fn)
	}
}

// isRunnable reports whether the command does work (has a Run/RunE) and is not
// the root. Grouping commands (no Run) are skipped in the reference body.
func isRunnable(cmd *cobra.Command) bool {
	return cmd.Parent() != nil && (cmd.Run != nil || cmd.RunE != nil)
}

// depthBelow returns how many levels cmd sits below ancestor: 0 if cmd is the
// ancestor itself, 1 for a direct child, and so on.
func depthBelow(ancestor, cmd *cobra.Command) int {
	depth := 0
	for c := cmd; c != nil && c != ancestor; c = c.Parent() {
		depth++
	}
	return depth
}

// commandPath returns the space-delimited path below the root, e.g. "stack create".
func commandPath(cmd *cobra.Command) string {
	var parts []string
	for c := cmd; c.Parent() != nil; c = c.Parent() {
		parts = append([]string{c.Name()}, parts...)
	}
	return strings.Join(parts, " ")
}

func flagDocs(cmd *cobra.Command) []FlagDoc {
	var flags []FlagDoc
	seen := map[string]bool{}
	collect := func(flag *pflag.Flag) {
		if flag.Hidden || seen[flag.Name] {
			return
		}
		seen[flag.Name] = true
		flags = append(flags, FlagDoc{
			Name:      flag.Name,
			Shorthand: flag.Shorthand,
			Type:      flag.Value.Type(),
			Default:   nonEmptyDefault(flag),
			Required:  flagRequired(flag),
			Usage:     flag.Usage,
		})
	}
	cmd.LocalFlags().VisitAll(collect)
	cmd.InheritedFlags().VisitAll(collect)
	return flags
}

func nonEmptyDefault(flag *pflag.Flag) interface{} {
	if flag.DefValue == "" || (flag.DefValue == "false" && flag.Value.Type() == "bool") {
		return nil
	}
	return flag.DefValue
}

func flagRequired(flag *pflag.Flag) bool {
	if flag.Annotations == nil {
		return false
	}
	_, ok := flag.Annotations["cobra_annotation_bash_completion_one_required_flag"]
	return ok
}

func title(root *cobra.Command, cfg *DocsConfig) string {
	if cfg != nil && cfg.Title != "" {
		return cfg.Title
	}
	return root.Name()
}

func description(root *cobra.Command, cfg *DocsConfig) string {
	if cfg != nil && cfg.Description != "" {
		return cfg.Description
	}
	if root.Long != "" {
		return root.Long
	}
	return root.Short
}
