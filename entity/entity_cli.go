package entity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/flanksource/clicky/flags"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// GenerateCLI creates cobra subcommands for all registered entities under parent.
// Admin entities are nested under a shared "admin" parent command.
// Entities with a Parent set are nested under a shared parent cobra command,
// created lazily if it does not already exist.
func GenerateCLI(parent *cobra.Command) {
	adminCmds := make(map[string]*cobra.Command)
	for _, entity := range GetEntities() {
		target := parent
		if entity.Parent != "" {
			target = findOrCreateChild(parent, entity.Parent)
		}
		if entity.IsAdmin {
			key := target.CommandPath()
			adminCmd := adminCmds[key]
			if adminCmd == nil {
				adminCmd = &cobra.Command{
					Use:   "admin",
					Short: "Administrative operations",
				}
				target.AddCommand(adminCmd)
				adminCmds[key] = adminCmd
			}
			generateEntityCLI(adminCmd, entity)
		} else {
			generateEntityCLI(target, entity)
		}
	}

	flushPendingSubCommands(parent)
}

// findOrCreateChild returns the child command of parent named name. If no
// matching child exists, a thin parent command is created and attached.
// findOrCreateEntityCommand returns the command an entity's operations hang
// from, reusing one the application already registered under that name.
func findOrCreateEntityCommand(parent *cobra.Command, entity EntityInfo) (cmd *cobra.Command, created bool) {
	for _, candidate := range parent.Commands() {
		if candidate.Name() == entity.Name {
			annotateEntityCommand(candidate, entity)
			return candidate, false
		}
	}
	cmd = &cobra.Command{
		Use:     entity.Name,
		Aliases: entity.Aliases,
		Short:   fmt.Sprintf("Manage %s resources", entity.Name),
	}
	annotateEntityCommand(cmd, entity)
	parent.AddCommand(cmd)
	return cmd, true
}

// commandSet snapshots which subcommands a command already has.
func commandSet(cmd *cobra.Command) map[*cobra.Command]bool {
	set := map[*cobra.Command]bool{}
	for _, child := range cmd.Commands() {
		set[child] = true
	}
	return set
}

func findOrCreateChild(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	child := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("%s operations", name),
	}
	parent.AddCommand(child)
	return child
}

func generateEntityCLI(parent *cobra.Command, entity EntityInfo) {
	// An application may already have a command group of this name carrying
	// commands the entity does not own. Joining it keeps one group in help;
	// adding a second would leave the user two, only one of which works.
	entityCmd, ownsGroup := findOrCreateEntityCommand(parent, entity)
	existing := commandSet(entityCmd)

	for _, op := range entity.Operations {
		generateEntitySubcommand(entityCmd, entity, op)
	}

	if entity.PrimaryAction != nil {
		generatePrimaryAction(entityCmd, *entity.PrimaryAction)
	} else {
		promoteListToEntityRoot(entityCmd)
	}

	for _, action := range entity.Actions {
		scope := "entity"
		if action.OptionalID {
			scope = "collection"
		}
		generateIDCommand(entityCmd, idCommand{
			Verb:  action.Name,
			Short: action.Short,
			Op: EntityOperation{
				Verb:              action.Name,
				Method:            action.Method,
				RoutePath:         action.RoutePath,
				DataFunc:          action.DataFunc,
				ContextDataFunc:   action.ContextDataFunc,
				LookupFunc:        action.LookupFunc,
				ContextLookupFunc: action.ContextLookupFunc,
				BindCompletions:   action.BindCompletions,
				FlagSchemas:       action.FlagSchemas,
				FlagsType:         action.FlagsType,
				ResponseType:      action.ResponseType,
				Schedule:          action.Schedule,
			},
			ValidArgs:      entity.ValidArgs,
			MetaVerb:       "action",
			Scope:          scope,
			ActionName:     action.Name,
			IDParam:        "id",
			SupportsLookup: action.LookupFunc != nil || action.ContextLookupFunc != nil,
			OptionalID:     action.OptionalID,
			ToolHints:      action.ToolHints,
		})
	}

	for _, ba := range entity.BulkActions {
		generateBulkActionCommand(entityCmd, ba)
	}
	commandNames := make(map[string]bool)
	for _, child := range entityCmd.Commands() {
		commandNames[child.Name()] = true
	}
	for _, child := range entityCmd.Commands() {
		aliases := child.Aliases[:0]
		for _, alias := range child.Aliases {
			if !commandNames[alias] {
				aliases = append(aliases, alias)
			}
		}
		child.Aliases = aliases
	}
	storeIDOptions(entityCmd, existing, entity.IDOptions)

	if entity.ServedOnly {
		// Only the commands this call produced: a shared group's other commands
		// belong to the application, and hiding them would be a regression.
		for _, generated := range entityCmd.Commands() {
			if !existing[generated] {
				MarkServedOnly(generated)
			}
		}
		// A group this entity created holds nothing else, so leaving it visible
		// would advertise a command with no subcommands under it.
		if ownsGroup {
			MarkServedOnly(entityCmd)
		}
	}
}

// promoteListToEntityRoot makes the bare entity command (`xero-cli accounts`)
// run its own `list` subcommand, so the entity root mirrors the CLI's natural
// "type the entity, get the collection" UX and the RPC converter exposes it as
// the canonical GET /api/v1/<entity> (it then skips the now-redundant `list`
// endpoint — see rpc.Converter.shouldIncludeCommand).
//
// Only the list command's LOCAL flags (its ListOpts filters) are copied onto
// the root. The root already inherits every persistent flag through cobra's
// flag chain, so copying the merged set would re-declare globals like --format
// /--config/--loglevel as local flags — which the RPC converter would then
// surface as bogus query parameters on the operation. Copy locals only.
func promoteListToEntityRoot(entityCmd *cobra.Command) {
	var listCmd *cobra.Command
	for _, sub := range entityCmd.Commands() {
		if meta := GetCommandOpenAPIMeta(sub); meta != nil && meta.Verb == "list" {
			listCmd = sub
			break
		}
	}
	if listCmd == nil {
		return
	}

	entityCmd.RunE = listCmd.RunE
	entityCmd.Args = listCmd.Args
	entityCmd.ValidArgsFunction = listCmd.ValidArgsFunction
	entityCmd.Flags().AddFlagSet(listCmd.LocalNonPersistentFlags())

	// The root keeps its entity annotations with NO operation verb: the RPC
	// converter detects "runnable entity-root, no verb" to map it to GET
	// /api/v1/<entity> and to skip the redundant `list` subcommand. Only the
	// lookup-support flag is forwarded so the spec still advertises filter
	// lookups for the promoted endpoint.
	if listMeta := GetCommandOpenAPIMeta(listCmd); listMeta != nil && listMeta.SupportsLookup {
		setCommandAnnotation(entityCmd, annotationClickySupportsLookup, "true")
	}
	// The promoted root IS the list operation, so it inherits list's declared
	// permission and its safety hints. The hints matter more than they used to:
	// with verb defaults gone, they are what lets a consumer's policy resolve an
	// unset permission to auto-run. Forwarding only the permission would leave the
	// root looking like an unannotated command and silently downgrade it to "ask".
	for _, annotation := range []string{
		annotationClickyToolPermission,
		annotationClickyToolReadOnlyHint,
		annotationClickyToolDestructive,
		annotationClickyToolIdempotent,
	} {
		if entityCmd.Annotations[annotation] == "" {
			setCommandAnnotation(entityCmd, annotation, listCmd.Annotations[annotation])
		}
	}

	if df := GetDataFunc(listCmd); df != nil {
		dataFuncRegistry.Store(entityCmd, df)
	}
	if cdf := GetContextDataFunc(listCmd); cdf != nil {
		contextDataFuncRegistry.Store(entityCmd, cdf)
	}
	if lf := GetLookupFunc(listCmd); lf != nil {
		lookupFuncRegistry.Store(entityCmd, lf)
	}
	if clf := GetContextLookupFunc(listCmd); clf != nil {
		contextLookupFuncRegistry.Store(entityCmd, clf)
	}
	if meta := GetCommandResponseMeta(listCmd); meta != nil {
		SetCommandResponseMeta(entityCmd, *meta)
	}
}

func generateEntitySubcommand(parent *cobra.Command, entity EntityInfo, op EntityOperation) {
	switch op.Verb {
	case "list":
		generateListCommand(parent, entity, op)
	case "get":
		generateIDCommand(parent, idCommand{
			Verb:      "get",
			Short:     fmt.Sprintf("Get a %s by ID", entity.Name),
			Op:        op,
			ValidArgs: entity.ValidArgs,
			MetaVerb:  "get",
			Scope:     "entity",
			IDParam:   "id",
		})
	case "create":
		generateBodyCommand(parent, "create", fmt.Sprintf("Create a %s", entity.Name), op)
	case "update":
		generateBodyCommand(parent, "update", fmt.Sprintf("Update a %s", entity.Name), op)
	case "delete":
		generateIDCommand(parent, idCommand{
			Verb:      "delete",
			Short:     fmt.Sprintf("Delete a %s", entity.Name),
			Op:        op,
			ValidArgs: entity.ValidArgs,
			MetaVerb:  "delete",
			Scope:     "entity",
			IDParam:   "id",
		})
	}
}

// runEntityOp invokes the operation's data function on the CLI path, preferring
// the context-aware variant (fed cmd.Context()) when present.
func runEntityOp(c *cobra.Command, op EntityOperation, flagMap map[string]string, args []string) (any, error) {
	var (
		result any
		err    error
	)
	if op.ContextDataFunc != nil {
		result, err = op.ContextDataFunc(ContextWithOperationSurface(c.Context(), "cli"), flagMap, args)
	} else {
		result, err = op.DataFunc(flagMap, args)
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

// storeEntityDataFuncs records the operation's data closures against the command
// so the RPC converter can wire them onto the generated RPCOperation. The
// context-aware variant is stored separately and preferred downstream.
func storeEntityDataFuncs(cmd *cobra.Command, op EntityOperation) {
	if op.ContextDataFunc != nil {
		contextDataFuncRegistry.Store(cmd, ContextDataFunc(op.ContextDataFunc))
	}
	if op.DataFunc != nil {
		dataFuncRegistry.Store(cmd, op.DataFunc)
	}
}

func generateListCommand(parent *cobra.Command, entity EntityInfo, op EntityOperation) {
	cmd := &cobra.Command{
		Use:   "list",
		Short: fmt.Sprintf("List %s resources", entity.Name),
		RunE: func(c *cobra.Command, args []string) error {
			flagMap := make(map[string]string)
			c.Flags().Visit(func(f *pflag.Flag) {
				flagMap[f.Name] = flagMapValue(f)
			})
			result, err := runEntityOp(c, op, flagMap, args)
			if err != nil {
				return err
			}
			return RenderResult(result)
		},
	}

	// Bind filter flags from the ListOpts type
	bindTypeFlags(cmd, entity.ListType)
	bindSortFlags(cmd, op.Sort)
	if op.BindCompletions != nil {
		op.BindCompletions(cmd)
	}

	annotateEntityOperationCommand(cmd, parent, "list", op.Method, "collection", "", "", op.LookupFunc != nil, false, false, MCPToolHints{})
	setCommandAnnotation(cmd, annotationClickyOperationPath, op.RoutePath)
	parent.AddCommand(cmd)
	storeEntityDataFuncs(cmd, op)
	SetCommandResponseMeta(cmd, ResponseOpenAPIMeta{
		Type:     op.ResponseType,
		Array:    op.ResponseArray,
		Paged:    op.ResponsePaged,
		EntityID: op.ResponseEntityID,
	})
	if op.LookupFunc != nil {
		lookupFuncRegistry.Store(cmd, op.LookupFunc)
	}
	if op.ContextLookupFunc != nil {
		contextLookupFuncRegistry.Store(cmd, op.ContextLookupFunc)
	}
}

func bindSortFlags(cmd *cobra.Command, spec *SortSpec) {
	if spec == nil {
		return
	}
	for _, name := range []string{"sort", "order"} {
		if cmd.Flags().Lookup(name) != nil {
			panic(fmt.Sprintf("clicky sortable entity list options must not declare --%s", name))
		}
	}
	cmd.Flags().String("sort", spec.Default.Key, "Sort by a response column")
	cmd.Flags().String("order", string(spec.Default.Direction), "Sort direction")
	flags.SetAllowedValues(cmd.Flags().Lookup("sort"), spec.Keys()...)
	flags.SetAllowedValues(cmd.Flags().Lookup("order"), string(SortDirectionAsc), string(SortDirectionDesc))
}

// idCommand describes a generated subcommand that addresses one entity by id.
type idCommand struct {
	Verb               string
	Short              string
	Op                 EntityOperation
	ValidArgs          func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)
	MetaVerb           string
	Method             string
	Scope              string
	ActionName         string
	IDParam            string
	SupportsLookup     bool
	SupportsFilterMode bool
	OptionalID         bool
	ToolHints          MCPToolHints
}

func generateIDCommand(parent *cobra.Command, spec idCommand) {
	verb, op := spec.Verb, spec.Op
	hasFlags := op.FlagsType != nil
	idToken := "<id>"
	args := cobra.ExactArgs(1)
	if spec.OptionalID {
		idToken = "[id]"
		args = cobra.MaximumNArgs(1)
	}
	use := fmt.Sprintf("%s %s", verb, idToken)
	if hasFlags {
		use = fmt.Sprintf("%s %s [flags]", verb, idToken)
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: spec.Short,
		Args:  args,
		RunE: func(c *cobra.Command, args []string) error {
			var flagMap map[string]string
			if hasFlags {
				flagMap = make(map[string]string)
				c.Flags().Visit(func(f *pflag.Flag) {
					flagMap[f.Name] = flagMapValue(f)
				})
			}
			result, err := runEntityOp(c, op, flagMap, args)
			if err != nil {
				return renderCommandError(verb, err)
			}
			if result != nil {
				return RenderResult(result)
			}
			return nil
		},
	}
	if verb == "get" {
		cmd.Aliases = []string{"inspect"}
	}
	if spec.ValidArgs != nil {
		cmd.ValidArgsFunction = spec.ValidArgs
	}
	if hasFlags {
		bindTypeFlags(cmd, op.FlagsType)
	}
	if op.BindCompletions != nil {
		op.BindCompletions(cmd)
	}
	method := spec.Method
	if method == "" {
		method = op.Method
	}
	annotateEntityOperationCommand(cmd, parent, spec.MetaVerb, method, spec.Scope, spec.ActionName, spec.IDParam, spec.SupportsLookup, spec.SupportsFilterMode, spec.OptionalID, spec.ToolHints)
	setCommandAnnotation(cmd, annotationClickyOperationPath, op.RoutePath)
	if op.Schedule != nil {
		AnnotateSchedule(cmd, *op.Schedule)
	}
	parent.AddCommand(cmd)
	storeEntityDataFuncs(cmd, op)
	if op.LookupFunc != nil {
		lookupFuncRegistry.Store(cmd, op.LookupFunc)
	}
	if op.ContextLookupFunc != nil {
		contextLookupFuncRegistry.Store(cmd, op.ContextLookupFunc)
	}
	storeFlagSchemas(cmd, op.FlagSchemas)
	SetCommandResponseMeta(cmd, ResponseOpenAPIMeta{
		Type:     op.ResponseType,
		Array:    op.ResponseArray,
		Paged:    op.ResponsePaged,
		EntityID: op.ResponseEntityID,
	})
}

// actionFlagsType resolves an ActionFlags implementer to its concrete
// struct reflect.Type, dereferencing a pointer if the caller handed one in.
// Returns nil when the action declared no flags.
func actionFlagsType(f ActionFlags) reflect.Type {
	if f == nil {
		return nil
	}
	t := reflect.TypeOf(f)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func generateBodyCommand(parent *cobra.Command, verb, short string, op EntityOperation) {
	use, argsValidator := fmt.Sprintf("%s [key=value ...]", verb), cobra.ArbitraryArgs
	if verb == "update" {
		use, argsValidator = "update <id> [key=value ...]", cobra.MinimumNArgs(1)
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  argsValidator,
		RunE: func(c *cobra.Command, args []string) error {
			flagMap := make(map[string]string)
			c.Flags().Visit(func(f *pflag.Flag) {
				flagMap[f.Name] = flagMapValue(f)
			})

			// For update, first arg is ID
			callArgs := args
			if verb == "update" {
				flagMap["id"] = args[0]
				callArgs = args[1:]
			}

			if len(callArgs) > 0 {
				parsed, err := ParseArgs(callArgs)
				if err != nil {
					return err
				}
				for k, v := range parsed {
					flagMap[k] = fmt.Sprintf("%v", v)
				}
			} else if isStdinAvailable() {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				var body map[string]any
				if err := json.Unmarshal(data, &body); err != nil {
					return fmt.Errorf("parsing JSON from stdin: %w", err)
				}
				for k, v := range body {
					flagMap[k] = fmt.Sprintf("%v", v)
				}
			}
			if verb == "update" && flagMap["id"] != args[0] {
				return fmt.Errorf("update body id %q conflicts with positional id %q", flagMap["id"], args[0])
			}

			result, err := runEntityOp(c, op, flagMap, args)
			if err != nil {
				return err
			}
			if result != nil {
				return RenderResult(result)
			}
			return nil
		},
	}
	scope := "collection"
	idParam := ""
	if verb == "update" {
		scope = "entity"
		idParam = "id"
	}
	annotateEntityOperationCommand(cmd, parent, verb, op.Method, scope, "", idParam, false, false, false, MCPToolHints{})
	setCommandAnnotation(cmd, annotationClickyOperationPath, op.RoutePath)
	parent.AddCommand(cmd)
	storeEntityDataFuncs(cmd, op)
	SetCommandResponseMeta(cmd, ResponseOpenAPIMeta{
		Type:     op.ResponseType,
		Array:    op.ResponseArray,
		Paged:    op.ResponsePaged,
		EntityID: op.ResponseEntityID,
	})
}

func generateBulkActionCommand(parent *cobra.Command, ba BulkActionInfo) {
	execute := func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
		if flagMap["filter"] != "" {
			if ba.ContextFilterFunc != nil {
				return ba.ContextFilterFunc(ctx, flagMap, args)
			}
			if ba.FilterFunc != nil {
				return ba.FilterFunc(flagMap, args)
			}
		}
		if ba.ContextDataFunc != nil {
			return ba.ContextDataFunc(ctx, flagMap, args)
		}
		return ba.DataFunc(flagMap, args)
	}

	cmd := &cobra.Command{
		// The first id is written as an operand rather than folded into the
		// variadic tail because it is what the REST path is derived from: the
		// route is /entity/{id}/action and the selection rides comma-joined in
		// that segment. A bare "[id...]" is not a named operand, and the action
		// then generates a collection route with nowhere to put the selection.
		Use:   fmt.Sprintf("%s <id> [id...]", ba.Name),
		Short: ba.Short,
		Args:  cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, args []string) error {
			flagMap := make(map[string]string)
			c.Flags().Visit(func(f *pflag.Flag) {
				flagMap[f.Name] = flagMapValue(f)
			})
			if len(args) == 0 && flagMap["filter"] == "" {
				if ba.FilterFunc == nil && ba.ContextFilterFunc == nil {
					return fmt.Errorf("bulk action %q requires one or more ids", ba.Name)
				}
				return fmt.Errorf("bulk action %q requires one or more ids or --filter", ba.Name)
			}

			// Use filter mode if --filter flag is set and FilterFunc exists
			ctx := c.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			result, err := execute(ContextWithOperationSurface(ctx, "cli"), flagMap, args)
			if err != nil {
				return err
			}
			if result != nil {
				return RenderResult(result)
			}
			return nil
		},
	}

	if ba.FilterFunc != nil || ba.ContextFilterFunc != nil {
		bindTypeFlags(cmd, ba.ListType)
		if ba.BindCompletions != nil {
			ba.BindCompletions(cmd)
		}
	}
	// The action's own parameters bind after the selection flags, skipping any
	// name the ListOpts already claimed: pflag panics on a redefined flag, and a
	// selector and an operation can legitimately both want e.g. `--status`. The
	// selector wins because it is the one the entity declared.
	bindTypeFlagsSkippingExisting(cmd, ba.FlagsType)

	annotateEntityOperationCommand(cmd, parent, "action", "", "collection", ba.Name, "id", ba.LookupFunc != nil, ba.FilterFunc != nil || ba.ContextFilterFunc != nil, false, ba.ToolHints)
	parent.AddCommand(cmd)
	if ba.ContextDataFunc != nil || ba.ContextFilterFunc != nil {
		contextDataFuncRegistry.Store(cmd, ContextDataFunc(execute))
	}
	dataFuncRegistry.Store(cmd, func(flagMap map[string]string, args []string) (any, error) {
		return execute(context.Background(), flagMap, args)
	})
	SetCommandResponseMeta(cmd, ResponseOpenAPIMeta{Type: ba.ResponseType})
	if ba.LookupFunc != nil {
		lookupFuncRegistry.Store(cmd, ba.LookupFunc)
	}
	if ba.ContextLookupFunc != nil {
		contextLookupFuncRegistry.Store(cmd, ba.ContextLookupFunc)
	}
}

// bindTypeFlags registers cobra flags from a struct type's field tags.
func bindTypeFlags(cmd *cobra.Command, t reflect.Type) {
	for _, info := range mustParseFlagFields(t) {
		flags.BindFlag(cmd, info)
	}
}

// bindTypeFlagsSkippingExisting is bindTypeFlags for a second struct bound onto
// the same command, dropping fields whose flag name is already registered.
// pflag panics rather than reports on a redefinition, so a bulk action whose
// parameters happen to overlap its entity's filters would take the process down
// at startup instead of at the call site that could fix it.
func bindTypeFlagsSkippingExisting(cmd *cobra.Command, t reflect.Type) {
	if t == nil {
		return
	}
	for _, info := range mustParseFlagFields(t) {
		if info.FlagName != "" && cmd.Flags().Lookup(info.FlagName) != nil {
			continue
		}
		flags.BindFlag(cmd, info)
	}
}

// mustParseFlagFields panics on malformed flag tags: they are programmer errors
// in a registered type, and CLI generation has no error path to report them.
func mustParseFlagFields(t reflect.Type) []flags.FieldInfo {
	fieldInfos, err := flags.ParseStructFields(t)
	if err != nil {
		panic(fmt.Sprintf("failed to parse flag fields of %s: %v", t, err))
	}
	return fieldInfos
}
