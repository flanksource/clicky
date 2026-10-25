package rpc

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/entity"
)

const interactiveFlag = "interactive"

// InstallInteractive adds a persistent -i/--interactive flag (without -i only
// when a command uses -i for a differently named flag) to root and wires every runnable
// command in the tree: with the flag set, the command's argument form opens
// pre-filled with the flags and args already given (PromptOperation), the
// answers are written back to the flags and positional args, and the command
// runs. A command that defines its own --interactive keeps it and is left
// unwired. Call it once the command tree is complete.
func InstallInteractive(root *cobra.Command) {
	if root.PersistentFlags().Lookup(interactiveFlag) != nil || root.Flags().Lookup(interactiveFlag) != nil {
		panic(fmt.Sprintf("rpc.InstallInteractive: %s already defines --%s", root.CommandPath(), interactiveFlag))
	}
	// A command's own --interactive shadows the global one entirely (cobra only
	// inherits a flag whose name the command does not define), so its -i never
	// collides. Only -i on a differently named flag (--input, --id) does, and
	// cobra would panic merging it, so then the global flag goes without -i.
	shorthand := "i"
	visitCommands(root, func(cmd *cobra.Command) {
		for _, set := range []*pflag.FlagSet{cmd.Flags(), cmd.PersistentFlags()} {
			if f := set.ShorthandLookup("i"); f != nil && f.Name != interactiveFlag {
				shorthand = ""
			}
		}
	})
	visitCommands(root, func(cmd *cobra.Command) {
		if cmd.Runnable() && cmd.LocalFlags().Lookup(interactiveFlag) == nil {
			wrapInteractive(cmd)
		}
	})
	root.PersistentFlags().BoolP(interactiveFlag, shorthand, false, "Fill in the command's arguments with an interactive form")
}

func visitCommands(cmd *cobra.Command, visit func(*cobra.Command)) {
	visit(cmd)
	for _, child := range cmd.Commands() {
		visitCommands(child, visit)
	}
}

func interactiveRequested(cmd *cobra.Command) bool {
	on, err := cmd.Flags().GetBool(interactiveFlag)
	return err == nil && on
}

// wrapInteractive defers positional-arg validation while --interactive is set
// (the form supplies the args), fills the flags in PreRunE — before cobra
// checks required flags — and hands the filled args to the original Run.
func wrapInteractive(cmd *cobra.Command) {
	validateArgs := cmd.Args
	if validateArgs != nil {
		cmd.Args = func(c *cobra.Command, args []string) error {
			if interactiveRequested(c) {
				return nil
			}
			return validateArgs(c, args)
		}
	}
	preRunE, preRun := cmd.PreRunE, cmd.PreRun
	runE, run := cmd.RunE, cmd.Run
	var filled []string
	interactiveRun := false
	cmd.PreRun = nil
	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		interactiveRun = false
		if interactiveRequested(c) {
			next, err := fillInteractive(c, args, validateArgs)
			if err != nil {
				return err
			}
			if validateArgs != nil {
				if err := validateArgs(c, next); err != nil {
					return err
				}
			}
			filled, interactiveRun, args = next, true, next
		}
		if preRunE != nil {
			return preRunE(c, args)
		}
		if preRun != nil {
			preRun(c, args)
		}
		return nil
	}
	cmd.Run = nil
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if interactiveRun {
			args = filled
		}
		if runE != nil {
			return runE(c, args)
		}
		run(c, args)
		return nil
	}
}

// fillInteractive runs the operation form for cmd and applies the answers.
// validateArgs is the command's own Args, which the -i wrapper hides from the
// converter: rejecting no args makes the positional required.
func fillInteractive(cmd *cobra.Command, args []string, validateArgs cobra.PositionalArgs) ([]string, error) {
	op, err := NewConverter(DefaultConfig()).ConvertCommand(cmd)
	if err != nil {
		return nil, err
	}
	withPositional(cmd, op)
	var positional []string
	for i, param := range op.Parameters {
		if cmd.Flags().Lookup(param.Name) != nil {
			continue
		}
		positional = append(positional, param.Name)
		if validateArgs != nil && validateArgs(cmd, nil) != nil {
			op.Parameters[i].Required = true
		}
	}
	preset, err := interactivePreset(cmd, *op, args)
	if err != nil {
		return nil, err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	completions, err := commandCompletions(cmd, *op, args)
	if err != nil {
		return nil, err
	}
	values, err := PromptOperation(ctx, *op, PromptOperationOptions{Preset: preset, Completions: completions, Positional: positional, FlagSchemas: entity.GetFlagSchemas(cmd)})
	if err != nil {
		return nil, fmt.Errorf("%s --%s: %w", cmd.CommandPath(), interactiveFlag, err)
	}
	return applyInteractive(cmd, *op, args, values)
}

// withPositional adds the command's positional argument to its form when the
// operation has none: an optional-id action (`prompt run [id]`) takes no id
// over HTTP, so the operation omits it, but on the CLI it is the argument.
func withPositional(cmd *cobra.Command, op *RPCOperation) {
	if !clicky.AcceptsPositionalArgs(cmd) {
		return
	}
	for _, param := range op.Parameters {
		if cmd.Flags().Lookup(param.Name) == nil {
			return
		}
	}
	param := RPCParameter{Name: "args", Type: "array", Description: "Positional arguments for the command", In: "query", Positional: true}
	if name := extractParameterName(cmd.Use); name != "" {
		param = RPCParameter{Name: name, Type: "string", Description: "Positional argument from command", In: "query", Positional: true}
	}
	op.Parameters = append([]RPCParameter{param}, op.Parameters...)
}

// interactivePreset is the form's starting value: every flag the user passed
// and the positional args.
func interactivePreset(cmd *cobra.Command, op RPCOperation, args []string) (map[string]any, error) {
	preset := map[string]any{}
	for _, param := range op.Parameters {
		flag := cmd.Flags().Lookup(param.Name)
		switch {
		case flag == nil && param.Name == "args":
			if len(args) > 0 {
				preset["args"] = toAnySlice(args)
			}
		case flag == nil:
			if len(args) > 0 {
				preset[param.Name] = args[0]
			}
		case flag.Changed:
			v, err := flagValue(flag, param.Type)
			if err != nil {
				return nil, fmt.Errorf("--%s: %w", flag.Name, err)
			}
			preset[param.Name] = v
		}
	}
	return preset, nil
}

func flagValue(flag *pflag.Flag, typ string) (any, error) {
	if slice, ok := flag.Value.(pflag.SliceValue); ok {
		return toAnySlice(slice.GetSlice()), nil
	}
	switch typ {
	case "boolean":
		return strconv.ParseBool(flag.Value.String())
	case "integer", "number":
		return strconv.ParseFloat(flag.Value.String(), 64)
	}
	return flag.Value.String(), nil
}

// applyInteractive writes the answers back: flags are set (or restored to their
// default when the answer left them empty) and positional args replaced.
func applyInteractive(cmd *cobra.Command, op RPCOperation, args []string, values map[string]any) ([]string, error) {
	next := args
	known := map[string]bool{}
	for _, param := range op.Parameters {
		known[param.Name] = true
		value, answered := values[param.Name]
		flag := cmd.Flags().Lookup(param.Name)
		switch {
		case flag == nil && param.Name == "args":
			next = nil
			if answered {
				next = operationArgs(value)
			}
		case flag == nil:
			rest := []string{}
			if len(args) > 1 {
				rest = args[1:]
			}
			next = rest
			if answered {
				next = append([]string{convertValueToString(value)}, rest...)
			}
		default:
			if err := setFlag(cmd, flag, value, answered); err != nil {
				return nil, err
			}
		}
	}
	for name := range values {
		if !known[name] {
			return nil, fmt.Errorf("%s: the form returned unknown argument %q", cmd.CommandPath(), name)
		}
	}
	return next, nil
}

func setFlag(cmd *cobra.Command, flag *pflag.Flag, value any, answered bool) error {
	// An answer that is just the flag's untouched default leaves the flag
	// unchanged, so commands that check Changed see what the user really set.
	if !flag.Changed && (!answered || convertValueToString(value) == strings.Trim(flag.DefValue, "[]")) {
		return nil
	}
	if slice, ok := flag.Value.(pflag.SliceValue); ok {
		items := []string{}
		if answered {
			items = operationArgs(value)
		}
		if err := slice.Replace(items); err != nil {
			return fmt.Errorf("--%s: %w", flag.Name, err)
		}
		flag.Changed = answered
		return nil
	}
	if !answered {
		if err := flag.Value.Set(flag.DefValue); err != nil {
			return fmt.Errorf("--%s: restore default: %w", flag.Name, err)
		}
		flag.Changed = false
		return nil
	}
	if err := cmd.Flags().Set(flag.Name, convertValueToString(value)); err != nil {
		return fmt.Errorf("--%s: %w", flag.Name, err)
	}
	return nil
}

func toAnySlice(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}
