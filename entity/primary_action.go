package entity

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// PrimaryActionWithContext creates the typed collection action that runs when
// the bare entity command is invoked. Its POST route shares the entity's
// collection path with the list operation's GET route.
func PrimaryActionWithContext[Opts ActionFlags, R any](opts Opts, fn func(context.Context, Opts) (R, error)) *ActionSpec[R] {
	return ActionWithFlagsAndContext("run", opts, func(ctx context.Context, _ string, flagMap map[string]string) (R, error) {
		resolved, err := buildOpts[Opts](flagMap)
		if err != nil {
			var zero R
			return zero, err
		}
		return fn(ctx, resolved)
	}).WithOptionalID().WithMethod("POST")
}

func generatePrimaryAction(entityCmd *cobra.Command, action ActionInfo) {
	op := EntityOperation{
		Verb:            "action",
		Method:          action.Method,
		DataFunc:        action.DataFunc,
		ContextDataFunc: action.ContextDataFunc,
		FlagsType:       action.FlagsType,
		ResponseType:    action.ResponseType,
	}
	entityCmd.Args = cobra.NoArgs
	entityCmd.RunE = func(c *cobra.Command, args []string) error {
		flagMap := make(map[string]string)
		c.Flags().Visit(func(f *pflag.Flag) {
			flagMap[f.Name] = flagMapValue(f)
		})
		result, err := runEntityOp(c, op, flagMap, args)
		if err != nil {
			return err
		}
		if result != nil {
			return RenderResult(result)
		}
		return nil
	}
	if action.Short != "" {
		entityCmd.Short = action.Short
	}
	if action.FlagsType != nil {
		bindTypeFlags(entityCmd, action.FlagsType)
	}
	annotateEntityOperationCommand(entityCmd, entityCmd, "action", action.Method, "collection", action.Name, "", false, false, true, action.ToolHints)
	storeEntityDataFuncs(entityCmd, op)
	SetCommandResponseMeta(entityCmd, ResponseOpenAPIMeta{Type: action.ResponseType})
}
