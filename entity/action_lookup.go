package entity

import (
	"context"

	"github.com/spf13/cobra"
)

// ActionIDSetter receives the entity ID while Clicky resolves a typed action's
// flags and lookups. It lets dependent filters load options for the entity in
// the action path without exposing that path parameter as a duplicate flag.
type ActionIDSetter interface {
	SetClickyActionID(id string)
}

// ActionContextSetter is the request-aware form of ActionIDSetter. Typed
// context actions use it when filters need both the entity ID and cancellation
// or request-scoped values while resolving dependent lookup options.
type ActionContextSetter interface {
	SetClickyActionContext(ctx context.Context, id string)
}

// WithActionFilters attaches the filters of an action's flags struct to an
// action built from a raw flag map (ActionWithFlags*): they serve the action's
// lookup (`?__lookup=filters`, the web and terminal forms) and its flags' shell
// completion, exactly as TypedActionWithContext wires them for a typed action.
func WithActionFilters[Opts ActionFlags, R any](action *ActionSpec[R], filters []Filter[Opts]) *ActionSpec[R] {
	if len(filters) == 0 {
		return action
	}
	action.lookupFunc = buildActionLookupFunc(filters)
	action.contextLookupFunc = buildActionLookupFuncWithContext(filters)
	action.bindCompletions = buildActionFilterCompletionBinder(filters)
	return action
}

func buildActionFilterCompletionBinder[T any](filters []Filter[T]) func(cmd *cobra.Command) {
	return buildFilterCompletionBinderWithOptions(filters, func(ctx context.Context, args []string, opts *T) {
		setActionContext(ctx, opts, actionIDFrom(nil, args))
	})
}

func buildActionLookupFunc[T any](filters []Filter[T]) func(flags map[string]string, args []string) (any, error) {
	if len(filters) == 0 {
		return nil
	}
	spec := newLookupSpec(filters)
	return func(flagMap map[string]string, args []string) (any, error) {
		return resolveActionLookup(context.Background(), spec, flagMap, args)
	}
}

func buildActionLookupFuncWithContext[T any](filters []Filter[T]) func(ctx context.Context, flags map[string]string, args []string) (any, error) {
	if len(filters) == 0 {
		return nil
	}
	spec := newLookupSpec(filters)
	return func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
		return resolveActionLookup(ctx, spec, flagMap, args)
	}
}

func resolveActionLookup[T any](ctx context.Context, spec lookupSpec[T], flagMap map[string]string, args []string) (any, error) {
	search := takeLookupSearch(flagMap)
	opts, err := buildOpts[T](flagMap)
	if err != nil {
		return nil, err
	}
	setActionContext(ctx, &opts, actionIDFrom(flagMap, args))
	return resolveLookupOptions(ctx, spec, opts, search)
}

func resolveActionOpts[T any](ctx context.Context, id string, flagMap map[string]string, filters []Filter[T]) (T, error) {
	opts, err := buildOpts[T](flagMap)
	if err != nil {
		return opts, err
	}
	setActionContext(ctx, &opts, id)
	_, err = applyEntityFilters(&opts, filters)
	return opts, err
}

func actionIDFrom(flagMap map[string]string, args []string) string {
	if id := flagMap["id"]; id != "" {
		return id
	}
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

func setActionContext[T any](ctx context.Context, opts *T, id string) {
	if setter, ok := any(opts).(ActionContextSetter); ok {
		setter.SetClickyActionContext(ctx, id)
		return
	}
	if setter, ok := any(opts).(ActionIDSetter); ok {
		setter.SetClickyActionID(id)
	}
}
