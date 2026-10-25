package rpc

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/schemaform"
)

// Completion lists the choices for one argument given the form's answers so
// far — the command's own shell completion, so any command with completion
// (an entity's ids, a filter's options) offers them in its form.
type Completion func(ctx context.Context, values map[string]any) ([]schemaform.Option, error)

// commandCompletions returns a Completion for every argument of cmd whose shell
// completion offers choices now: the positional arg's ValidArgsFunction or
// ValidArgs, and each flag's registered completion func. Completions that offer
// files or directories are not choices and are left out.
func commandCompletions(cmd *cobra.Command, op RPCOperation, args []string) (map[string]Completion, error) {
	out := map[string]Completion{}
	for _, param := range op.Parameters {
		flag := cmd.Flags().Lookup(param.Name)
		// An entity's <id> offers its items with names and descriptions.
		if ids := entity.GetIDOptions(cmd); flag == nil && ids != nil {
			out[param.Name] = func(ctx context.Context, _ map[string]any) ([]schemaform.Option, error) {
				return idOptions(ctx, ids)
			}
			continue
		}
		complete, positional := completionFunc(cmd, param.Name, flag == nil)
		if complete == nil {
			continue
		}
		argsFor := func(current []string) []string {
			if positional {
				return nil
			}
			return current
		}
		// A positional completes entity ids ("id<TAB>name"): the name is the
		// label. A flag's value is what users type (a model id), so it stays
		// visible next to its description.
		showValue := !positional
		probe, err := completionOptions(cmd, complete, argsFor(args), showValue)
		if err != nil {
			return nil, fmt.Errorf("%s: completing %s: %w", cmd.CommandPath(), param.Name, err)
		}
		if len(probe) == 0 {
			continue
		}
		out[param.Name] = func(_ context.Context, values map[string]any) ([]schemaform.Option, error) {
			// Completion reads the command's flags: sync them to the answers so
			// far so dependent completions narrow as the form is filled in.
			current, err := applyInteractive(cmd, op, args, values)
			if err != nil {
				return nil, err
			}
			return completionOptions(cmd, complete, argsFor(current), showValue)
		}
	}
	return out, nil
}

// idOptions lists an entity's ids as options labelled by name.
func idOptions(ctx context.Context, ids entity.IDOptionsFunc) ([]schemaform.Option, error) {
	items, err := ids(ctx)
	if err != nil {
		return nil, fmt.Errorf("list ids: %w", err)
	}
	options := make([]schemaform.Option, len(items))
	for i, item := range items {
		label := item.Name
		if label == "" {
			label = item.ID
		}
		options[i] = schemaform.Option{Value: item.ID, Label: label, Raw: item.ID, Description: item.Description}
	}
	return options, nil
}

func completionFunc(cmd *cobra.Command, name string, positional bool) (cobra.CompletionFunc, bool) {
	if !positional {
		complete, _ := cmd.GetFlagCompletionFunc(name)
		return complete, false
	}
	if cmd.ValidArgsFunction != nil {
		return cmd.ValidArgsFunction, true
	}
	if len(cmd.ValidArgs) > 0 {
		valid := cmd.ValidArgs
		return func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
			return valid, cobra.ShellCompDirectiveNoFileComp
		}, true
	}
	return nil, positional
}

// completionOptions runs a completion func and converts its "value\tdescription"
// entries into options labelled "description (value)", or just "description"
// when showValue is false.
func completionOptions(cmd *cobra.Command, complete cobra.CompletionFunc, args []string, showValue bool) ([]schemaform.Option, error) {
	entries, directive := complete(cmd, args, "")
	if directive&cobra.ShellCompDirectiveError != 0 {
		return nil, fmt.Errorf("shell completion reported an error")
	}
	if directive&(cobra.ShellCompDirectiveFilterFileExt|cobra.ShellCompDirectiveFilterDirs) != 0 {
		return nil, nil
	}
	options := make([]schemaform.Option, 0, len(entries))
	for _, entry := range entries {
		value, description, _ := strings.Cut(entry, "\t")
		if value = strings.TrimSpace(value); value == "" {
			continue
		}
		label := value
		switch description = strings.TrimSpace(description); {
		case description == "" || description == value:
		case showValue:
			label = description + " (" + value + ")"
		default:
			label = description
		}
		options = append(options, schemaform.Option{Value: value, Label: label, Raw: value})
	}
	return options, nil
}
