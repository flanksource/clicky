package entity

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/spf13/cobra"
)

// FlagSchema shapes a flag whose value is a JSON object that depends on the
// command's other arguments (a prompt's input variables, keyed by the prompt).
// Scope names the arguments (flags, or the positional id) it depends on; Load
// answers with the JSON Schema for their values, or nil when they select
// nothing that takes input. The interactive form (-i) renders the schema's
// fields in place of the flag and passes their answers as the flag's JSON.
type FlagSchema struct {
	Scope []string
	Load  func(ctx context.Context, values map[string]string) (json.RawMessage, error)
}

// flagSchemaRegistry maps generated commands to their flag schemas.
var flagSchemaRegistry sync.Map // map[*cobra.Command]map[string]FlagSchema

// WithActionFlagSchema declares that the action's flag takes a JSON object
// shaped by schema.
func WithActionFlagSchema[R any](action *ActionSpec[R], flag string, schema FlagSchema) *ActionSpec[R] {
	if schema.Load == nil || len(schema.Scope) == 0 {
		panic(fmt.Sprintf("entity.WithActionFlagSchema(%s --%s): a FlagSchema needs a Scope and a Load", action.name, flag))
	}
	if action.flagSchemas == nil {
		action.flagSchemas = map[string]FlagSchema{}
	}
	action.flagSchemas[flag] = schema
	return action
}

// GetFlagSchemas returns the flag schemas declared for a generated command.
func GetFlagSchemas(cmd *cobra.Command) map[string]FlagSchema {
	if v, ok := flagSchemaRegistry.Load(cmd); ok {
		return v.(map[string]FlagSchema)
	}
	return nil
}

func storeFlagSchemas(cmd *cobra.Command, schemas map[string]FlagSchema) {
	if len(schemas) == 0 {
		return
	}
	for flag := range schemas {
		if cmd.Flags().Lookup(flag) == nil {
			panic(fmt.Sprintf("%s: FlagSchema for --%s, which the command does not define", cmd.CommandPath(), flag))
		}
	}
	flagSchemaRegistry.Store(cmd, schemas)
}
