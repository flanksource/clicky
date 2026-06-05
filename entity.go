package clicky

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/flags"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	entityRegistry   []EntityInfo
	entityRegistryMu sync.Mutex
	multiFilterType  = reflect.TypeOf(MultiFilter{})
)

// flagMapValue returns the string form of f suitable for round-tripping
// through buildOpts (which calls flag.Value.Set). For slice-valued flags
// pflag's Value.String() wraps the CSV in brackets like "[a,b,c]", and
// re-Setting that string makes the brackets part of the first element.
// SliceValue gives us the underlying []string so we can re-encode as the
// plain CSV that Set expects.
func flagMapValue(f *pflag.Flag) string {
	if sv, ok := f.Value.(pflag.SliceValue); ok {
		items := sv.GetSlice()
		if len(items) == 0 {
			return ""
		}
		var b strings.Builder
		w := csv.NewWriter(&b)
		_ = w.Write(items)
		w.Flush()
		return strings.TrimSuffix(b.String(), "\n")
	}
	return f.Value.String()
}

// EntityItem is the interface that all entity types must implement.
type EntityItem interface {
	GetID() string
	GetName() string
}

// EntityInfo is the type-erased representation stored in the registry.
type EntityInfo struct {
	Name        string
	Parent      string
	Aliases     []string
	Type        reflect.Type
	ListType    reflect.Type
	Operations  []EntityOperation
	Actions     []ActionInfo
	BulkActions []BulkActionInfo
	ValidArgs   func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)
	IsAdmin     bool
}

// EntityOperation represents a single CRUD operation.
type EntityOperation struct {
	Verb     string // "list", "get", "create", "update", "delete"
	Method   string // Optional explicit HTTP method for generated RPC/OpenAPI routes.
	DataFunc func(flags map[string]string, args []string) (any, error)
	// ContextDataFunc, when set, is preferred over DataFunc by both the CLI run
	// path (fed cmd.Context()) and the RPC executor (fed r.Context()). Lets the
	// operation resolve request-scoped state instead of process globals.
	ContextDataFunc ContextDataFunc
	// FlagsType, when non-nil, binds typed flags from the given struct type
	// onto the generated cobra command and collects their values into the
	// flag map passed to DataFunc. Used by actions that implement
	// ActionFlags; ignored for the built-in CRUD verbs.
	FlagsType  reflect.Type
	LookupFunc func(flags map[string]string, args []string) (any, error)
	// ContextLookupFunc, when set, is the context-aware filter lookup closure.
	// The RPC converter wires it onto RPCOperation.ContextLookupFunc and the
	// executor prefers it over LookupFunc, feeding the request context.
	ContextLookupFunc ContextLookupFunc
	BindCompletions   func(cmd *cobra.Command)
	ResponseType      reflect.Type
	ResponseArray     bool
	ResponseEntityID  bool
}

// ActionInfo is the type-erased representation of a single-entity action.
type ActionInfo struct {
	Name     string
	Short    string
	Method   string
	DataFunc func(flags map[string]string, args []string) (any, error)
	// ContextDataFunc, when set, is preferred over DataFunc by both the CLI run
	// path (fed cmd.Context()) and the RPC executor (fed r.Context()), mirroring
	// the entity CRUD seam. Lets an action resolve request-scoped state instead
	// of process globals.
	ContextDataFunc ContextDataFunc
	// FlagsType, if non-nil, is the struct type whose `flag:"..."` tagged
	// fields are registered as cobra flags on the generated action command
	// and populated into the flag map passed to DataFunc.
	FlagsType    reflect.Type
	ResponseType reflect.Type
	// OptionalID, when true, makes the positional <id> argument optional on
	// the generated action command — the action is invokable with no
	// argument (and the `id` passed to the run func is empty). Use for
	// actions whose target is supplied entirely through flags.
	OptionalID bool
}

// BulkActionInfo is the type-erased representation of a bulk action.
type BulkActionInfo struct {
	Name              string
	Short             string
	DataFunc          func(flags map[string]string, args []string) (any, error)
	FilterFunc        func(flags map[string]string, args []string) (any, error)
	LookupFunc        func(flags map[string]string, args []string) (any, error)
	ContextLookupFunc ContextLookupFunc
	ListType          reflect.Type
	BindCompletions   func(cmd *cobra.Command)
	ResponseType      reflect.Type
}

// ActionFlags is a marker interface implemented by options structs that
// declare typed cobra flags for a clicky Action. When an Action's Flags
// field is non-nil, the concrete type of the value is reflected for its
// `flag:"..."` struct tags and those flags are bound to the generated
// action cobra command.
//
// Implementers attach a no-op `ClickyActionFlags()` method to an options
// struct, then pass a zero value of that struct in `Action.Flags`.
type ActionFlags interface {
	ClickyActionFlags()
}

// Filter resolves raw entity option values into the backend-specific shape and
// exposes UI metadata for the currently selected and available values.
type Filter[ListOpts any] interface {
	Key() string
	Label() string
	Lookup(opts *ListOpts) (map[string]api.Textable, error)
	Options(opts ListOpts) map[string]api.Textable
}

// SearchableFilter is an OPTIONAL extension a Filter may implement when its
// option set is too large to enumerate in one shot (e.g. a SQL DISTINCT column
// with thousands of values). The lookup handler type-asserts to it; filters
// that don't implement it keep the plain Options() behaviour unchanged.
//
// OptionsWithQuery returns at most limit options. An empty query means "the
// head set": the first limit options plus total = the true distinct count, so
// the UI can show "… and N more". A non-empty query returns up to limit options
// matching it (server-side search), letting the UI surface values that sort
// beyond the head. total is only meaningful for the head (query == "") call.
type SearchableFilter[ListOpts any] interface {
	OptionsWithQuery(opts ListOpts, query string, limit int) (options map[string]api.Textable, total int)
}

// ContextFilter is an OPTIONAL extension a Filter may implement when its
// option set is resolved from request-scoped state (e.g. a per-request
// database handle) rather than a process global. When the lookup handler runs
// with a context (the RPC path feeds r.Context(), the CLI feeds cmd.Context())
// it prefers OptionsWithContext over Options; filters that don't implement it
// fall back to the plain Options() path unchanged.
type ContextFilter[ListOpts any] interface {
	OptionsWithContext(ctx context.Context, opts ListOpts) map[string]api.Textable
}

// ContextSearchableFilter is the context-aware counterpart of
// SearchableFilter: a large/searchable filter whose head and server-side
// search both need request-scoped state. The lookup handler prefers it over
// OptionsWithQuery when a context is available.
type ContextSearchableFilter[ListOpts any] interface {
	OptionsWithQueryAndContext(ctx context.Context, opts ListOpts, query string, limit int) (options map[string]api.Textable, total int)
}

// Filterable is implemented by AddNamedCommand options structs that want to
// expose typed filter lookups (dropdowns/typeaheads on the web UI, shell
// completions on the CLI) on the subcommand itself rather than inheriting
// only the parent entity's Filters slice. The same Filter[T] values that back
// an Entity's Filters slice work here; AddNamedCommand stores a LookupFunc
// in the lookupFuncRegistry and binds completions whenever an opts struct
// satisfies this interface.
//
// Use this whenever a subcommand's flag surface diverges from its parent
// list view — typically when a positional argument pins one or more
// identifier filters and the subcommand only exposes the orthogonal
// (status, type, date range) lenses.
type Filterable[T any] interface {
	Filters() []Filter[T]
}

// LiftFilters adapts a Filter[Inner] slice so the same picker definitions
// work against an outer struct that embeds (or otherwise reaches) the inner
// filter struct. The project func extracts a *Inner from any *Outer the
// filter system hands us; lookup keys, labels, options, and selected-value
// reads/writes flow through unchanged. Use this to keep picker constructors
// scoped to the inner filter type while letting subcommand options structs —
// which also carry positional args, behaviour flags, etc. — satisfy
// Filterable[Outer].
func LiftFilters[Outer any, Inner any](
	filters []Filter[Inner],
	project func(*Outer) *Inner,
) []Filter[Outer] {
	out := make([]Filter[Outer], 0, len(filters))
	for _, f := range filters {
		out = append(out, liftedFilter[Outer, Inner]{inner: f, project: project})
	}
	return out
}

type liftedFilter[Outer any, Inner any] struct {
	inner   Filter[Inner]
	project func(*Outer) *Inner
}

func (l liftedFilter[Outer, Inner]) Key() string   { return l.inner.Key() }
func (l liftedFilter[Outer, Inner]) Label() string { return l.inner.Label() }
func (l liftedFilter[Outer, Inner]) Lookup(opts *Outer) (map[string]api.Textable, error) {
	return l.inner.Lookup(l.project(opts))
}
func (l liftedFilter[Outer, Inner]) Options(opts Outer) map[string]api.Textable {
	return l.inner.Options(*l.project(&opts))
}

// OptionsWithQuery forwards to the inner filter when it is searchable, so a
// lifted DistinctColumnFilter keeps its head/search behaviour. When the inner
// filter isn't searchable it degrades to Options() with len() as the total,
// matching the non-truncated path.
func (l liftedFilter[Outer, Inner]) OptionsWithQuery(opts Outer, query string, limit int) (map[string]api.Textable, int) {
	inner := *l.project(&opts)
	if s, ok := l.inner.(SearchableFilter[Inner]); ok {
		return s.OptionsWithQuery(inner, query, limit)
	}
	options := l.inner.Options(inner)
	return options, len(options)
}

// OptionsWithContext forwards to the inner filter's context-aware Options when
// it implements ContextFilter, so a lifted db-backed filter resolves the
// request handle. Otherwise it degrades to the non-context Options().
func (l liftedFilter[Outer, Inner]) OptionsWithContext(ctx context.Context, opts Outer) map[string]api.Textable {
	inner := *l.project(&opts)
	if c, ok := l.inner.(ContextFilter[Inner]); ok {
		return c.OptionsWithContext(ctx, inner)
	}
	return l.inner.Options(inner)
}

// OptionsWithQueryAndContext forwards to the inner filter's context-aware
// searchable Options when available, falling back to the non-context
// searchable path, then to Options(), mirroring OptionsWithQuery.
func (l liftedFilter[Outer, Inner]) OptionsWithQueryAndContext(ctx context.Context, opts Outer, query string, limit int) (map[string]api.Textable, int) {
	inner := *l.project(&opts)
	if c, ok := l.inner.(ContextSearchableFilter[Inner]); ok {
		return c.OptionsWithQueryAndContext(ctx, inner, query, limit)
	}
	if s, ok := l.inner.(SearchableFilter[Inner]); ok {
		return s.OptionsWithQuery(inner, query, limit)
	}
	options := l.inner.Options(inner)
	return options, len(options)
}

// EntityAction is the type-erased registration surface for custom entity
// actions. Use Action or ActionWithFlags to construct values.
type EntityAction interface {
	actionInfo() ActionInfo
}

type actionSpec[R any] struct {
	name       string
	short      string
	method     string
	run        func(id string, flags map[string]string) (R, error)
	runCtx     func(ctx context.Context, id string, flags map[string]string) (R, error)
	flags      ActionFlags
	optionalID bool
}

// Action creates a typed custom operation on a single entity by ID.
func Action[R any](name string, fn func(id string, flags map[string]string) (R, error)) *actionSpec[R] {
	return &actionSpec[R]{name: name, run: fn}
}

// ActionWithFlags creates a typed custom operation with typed action flags.
func ActionWithFlags[R any](name string, flags ActionFlags, fn func(id string, flags map[string]string) (R, error)) *actionSpec[R] {
	return &actionSpec[R]{name: name, flags: flags, run: fn}
}

// ActionWithContext creates a typed action whose run closure receives the
// request-scoped context (cmd.Context() on the CLI, r.Context() over HTTP), so
// it can resolve a per-request database/config instead of a process global.
func ActionWithContext[R any](name string, fn func(ctx context.Context, id string, flags map[string]string) (R, error)) *actionSpec[R] {
	return &actionSpec[R]{name: name, runCtx: fn}
}

// ActionWithFlagsAndContext is ActionWithContext with typed action flags.
func ActionWithFlagsAndContext[R any](name string, flags ActionFlags, fn func(ctx context.Context, id string, flags map[string]string) (R, error)) *actionSpec[R] {
	return &actionSpec[R]{name: name, flags: flags, runCtx: fn}
}

func (a *actionSpec[R]) WithShort(short string) *actionSpec[R] {
	a.short = short
	return a
}

func (a *actionSpec[R]) WithFlags(flags ActionFlags) *actionSpec[R] {
	a.flags = flags
	return a
}

// WithMethod overrides the inferred HTTP method for the generated RPC/OpenAPI
// action route. Leave empty to keep the default inference behavior.
func (a *actionSpec[R]) WithMethod(method string) *actionSpec[R] {
	a.method = method
	return a
}

// WithOptionalID makes the positional <id> argument optional on the
// generated action command. Use for actions whose target is supplied
// entirely through flags; the `id` passed to the run func is then empty.
func (a *actionSpec[R]) WithOptionalID() *actionSpec[R] {
	a.optionalID = true
	return a
}

func (a *actionSpec[R]) actionID(flagMap map[string]string, args []string) (string, error) {
	id := flagMap["id"]
	if id == "" && len(args) > 0 {
		id = args[0]
	}
	if id == "" && !a.optionalID {
		return "", fmt.Errorf("id is required")
	}
	return id, nil
}

func (a *actionSpec[R]) actionInfo() ActionInfo {
	info := ActionInfo{
		Name:         a.name,
		Short:        a.short,
		Method:       a.method,
		FlagsType:    actionFlagsType(a.flags),
		ResponseType: responseTypeOf[R](),
		OptionalID:   a.optionalID,
	}
	if a.runCtx != nil {
		info.ContextDataFunc = func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
			id, err := a.actionID(flagMap, args)
			if err != nil {
				return nil, err
			}
			return a.runCtx(ctx, id, flagMap)
		}
	} else {
		info.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
			id, err := a.actionID(flagMap, args)
			if err != nil {
				return nil, err
			}
			return a.run(id, flagMap)
		}
	}
	return info
}

// EntityBulkAction is the type-erased registration surface for custom bulk
// actions. Use BulkAction or BulkFilterAction to construct values.
type EntityBulkAction interface {
	bulkActionInfo(resolveOpts func(map[string]string) (any, error)) BulkActionInfo
}

type bulkActionSpec[R any] struct {
	name       string
	short      string
	run        func(ids []string, flags map[string]string) (R, error)
	filterFunc func(opts any, flags map[string]string) (R, error)
	listType   reflect.Type
}

// BulkAction creates a typed custom operation on multiple entity IDs.
func BulkAction[R any](name string, fn func(ids []string, flags map[string]string) (R, error)) *bulkActionSpec[R] {
	return &bulkActionSpec[R]{name: name, run: fn}
}

// BulkFilterAction creates a typed custom operation that runs against a typed
// filtered list selection instead of explicit IDs.
func BulkFilterAction[ListOpts any, R any](name string, fn func(opts ListOpts, flags map[string]string) (R, error)) *bulkActionSpec[R] {
	listType := reflect.TypeOf((*ListOpts)(nil)).Elem()
	return &bulkActionSpec[R]{
		name:     name,
		listType: listType,
		filterFunc: func(opts any, flagMap map[string]string) (R, error) {
			typed, ok := opts.(ListOpts)
			if !ok {
				var zero R
				return zero, fmt.Errorf("expected %s options, got %T", listType, opts)
			}
			return fn(typed, flagMap)
		},
	}
}

// BulkActionWithFilter creates a typed bulk action that supports both explicit
// IDs and filtered selections.
func BulkActionWithFilter[ListOpts any, R any](
	name string,
	run func(ids []string, flags map[string]string) (R, error),
	runFilter func(opts ListOpts, flags map[string]string) (R, error),
) *bulkActionSpec[R] {
	action := BulkFilterAction(name, runFilter)
	action.run = run
	return action
}

func (b *bulkActionSpec[R]) WithShort(short string) *bulkActionSpec[R] {
	b.short = short
	return b
}

func (b *bulkActionSpec[R]) bulkActionInfo(resolveOpts func(map[string]string) (any, error)) BulkActionInfo {
	info := BulkActionInfo{
		Name:         b.name,
		Short:        b.short,
		ListType:     b.listType,
		ResponseType: responseTypeOf[R](),
	}
	if b.run != nil {
		info.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
			return b.run(args, flagMap)
		}
	} else {
		info.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
			return nil, fmt.Errorf("bulk action %q requires filter mode", b.name)
		}
	}
	if b.filterFunc != nil {
		info.FilterFunc = func(flagMap map[string]string, args []string) (any, error) {
			opts, err := resolveOpts(flagMap)
			if err != nil {
				return nil, err
			}
			return b.filterFunc(opts, flagMap)
		}
	}
	return info
}

// Entity configures a CRUD resource. All function fields are optional.
// T is the type returned by List and must implement EntityItem.
// R is the type returned by Get/Create/Update.
type Entity[T EntityItem, ListOpts any, R any] struct {
	Name string
	// Parent, when set, nests the entity's command under a parent cobra command
	// with that name. The parent command is created lazily by GenerateCLI if it
	// does not already exist.
	Parent string
	// Aliases applied to the generated entity cobra command.
	Aliases []string
	List    func(opts ListOpts) ([]T, error)
	Get     func(id string) (R, error)
	// ListWithContext / GetWithContext / GetWithFlagsAndContext are context-aware
	// variants of List / Get / GetWithFlags. When set they take precedence over
	// their non-context counterparts and receive the request-scoped
	// context.Context (r.Context() over HTTP, cmd.Context() on the CLI). Use them
	// to resolve per-request state (e.g. a database/config bundle) instead of
	// reaching for process globals.
	ListWithContext        func(ctx context.Context, opts ListOpts) ([]T, error)
	GetWithContext         func(ctx context.Context, id string) (R, error)
	GetWithFlagsAndContext func(ctx context.Context, id string, flags map[string]string) (R, error)
	// GetFlags, when non-nil, is a zero-value struct value implementing
	// ActionFlags whose `flag:"..."` tagged fields are registered as
	// CLI flags on the generated `get` subcommand. The parsed values are
	// passed into GetWithFlags.
	//
	// GetWithFlags is mutually exclusive with Get: when both are set,
	// GetWithFlags wins. Set GetFlags to declare the flag schema and
	// GetWithFlags to receive the typed values.
	GetFlags     ActionFlags
	GetWithFlags func(id string, flags map[string]string) (R, error)
	Create       func(body map[string]any) (R, error)
	Update       func(id string, body map[string]any) (R, error)
	Delete       func(id string) error
	// CreateWithContext / UpdateWithContext / DeleteWithContext are context-aware
	// variants of Create / Update / Delete, mirroring the read-side seam above.
	// When set they take precedence and receive the request-scoped context, so
	// writes target the request's environment rather than a process global.
	CreateWithContext func(ctx context.Context, body map[string]any) (R, error)
	UpdateWithContext func(ctx context.Context, id string, body map[string]any) (R, error)
	DeleteWithContext func(ctx context.Context, id string) error
	Filters           []Filter[ListOpts]

	Actions     []EntityAction
	BulkActions []EntityBulkAction

	// Admin groups admin-only operations (inspect, configure, etc.)
	// that produce different views/columns from the main CRUD operations.
	// Generates subcommands under an "admin" subgroup and routes like /entity/admin/{verb}.
	Admin *Entity[T, ListOpts, R]

	// ValidArgs provides shell completion for the ID argument.
	ValidArgs func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)
}

// entityIDFrom resolves the entity id from the `id` flag or the first
// positional arg, erroring when neither is present.
func entityIDFrom(flagMap map[string]string, args []string) (string, error) {
	id := flagMap["id"]
	if id == "" && len(args) > 0 {
		id = args[0]
	}
	if id == "" {
		return "", fmt.Errorf("id is required")
	}
	return id, nil
}

// entityBody copies the flag map into the body map passed to Create.
func entityBody(flagMap map[string]string) map[string]any {
	body := make(map[string]any, len(flagMap))
	for k, v := range flagMap {
		body[k] = v
	}
	return body
}

// entityBodyExcludingID copies the flag map minus the `id` key into the body
// map passed to Update (the id is supplied separately).
func entityBodyExcludingID(flagMap map[string]string) map[string]any {
	body := make(map[string]any, len(flagMap))
	for k, v := range flagMap {
		if k != "id" {
			body[k] = v
		}
	}
	return body
}

// RegisterEntity registers a CRUD entity. Call during init().
// CLI commands and HTTP routes are generated later via GenerateCLI/GenerateRoutes.
func RegisterEntity[T EntityItem, ListOpts any, R any](e Entity[T, ListOpts, R]) {
	name := e.Name
	if name == "" {
		name = lo.KebabCase(reflect.TypeOf((*T)(nil)).Elem().Name())
	}

	info := EntityInfo{
		Name:      name,
		Parent:    e.Parent,
		Aliases:   e.Aliases,
		Type:      reflect.TypeOf((*T)(nil)).Elem(),
		ListType:  reflect.TypeOf((*ListOpts)(nil)).Elem(),
		ValidArgs: e.ValidArgs,
	}

	if e.ListWithContext != nil || e.List != nil {
		op := EntityOperation{
			Verb:              "list",
			LookupFunc:        buildLookupFunc[ListOpts](e.Filters),
			ContextLookupFunc: buildLookupFuncWithContext[ListOpts](e.Filters),
			BindCompletions:   buildFilterCompletionBinder[ListOpts](e.Filters),
			ResponseType:      reflect.TypeOf((*T)(nil)).Elem(),
			ResponseArray:     true,
			ResponseEntityID:  true,
		}
		if e.ListWithContext != nil {
			op.ContextDataFunc = func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
				opts, err := resolveEntityOpts[ListOpts](flagMap, e.Filters)
				if err != nil {
					return nil, err
				}
				items, err := e.ListWithContext(ctx, opts)
				if err != nil {
					return nil, err
				}
				return withEntityIDs(items), nil
			}
		} else {
			op.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
				opts, err := resolveEntityOpts[ListOpts](flagMap, e.Filters)
				if err != nil {
					return nil, err
				}
				items, err := e.List(opts)
				if err != nil {
					return nil, err
				}
				return withEntityIDs(items), nil
			}
		}
		info.Operations = append(info.Operations, op)
	}

	switch {
	case e.GetWithFlagsAndContext != nil:
		info.Operations = append(info.Operations, EntityOperation{
			Verb:         "get",
			FlagsType:    actionFlagsType(e.GetFlags),
			ResponseType: responseTypeOf[R](),
			ContextDataFunc: func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
				id, err := entityIDFrom(flagMap, args)
				if err != nil {
					return nil, err
				}
				return e.GetWithFlagsAndContext(ctx, id, flagMap)
			},
		})
	case e.GetWithFlags != nil:
		info.Operations = append(info.Operations, EntityOperation{
			Verb:         "get",
			FlagsType:    actionFlagsType(e.GetFlags),
			ResponseType: responseTypeOf[R](),
			DataFunc: func(flagMap map[string]string, args []string) (any, error) {
				id, err := entityIDFrom(flagMap, args)
				if err != nil {
					return nil, err
				}
				return e.GetWithFlags(id, flagMap)
			},
		})
	case e.GetWithContext != nil:
		info.Operations = append(info.Operations, EntityOperation{
			Verb:         "get",
			ResponseType: responseTypeOf[R](),
			ContextDataFunc: func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
				id, err := entityIDFrom(flagMap, args)
				if err != nil {
					return nil, err
				}
				return e.GetWithContext(ctx, id)
			},
		})
	case e.Get != nil:
		info.Operations = append(info.Operations, EntityOperation{
			Verb:         "get",
			ResponseType: responseTypeOf[R](),
			DataFunc: func(flagMap map[string]string, args []string) (any, error) {
				id, err := entityIDFrom(flagMap, args)
				if err != nil {
					return nil, err
				}
				return e.Get(id)
			},
		})
	}

	if e.CreateWithContext != nil || e.Create != nil {
		op := EntityOperation{Verb: "create", ResponseType: responseTypeOf[R]()}
		if e.CreateWithContext != nil {
			op.ContextDataFunc = func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
				return e.CreateWithContext(ctx, entityBody(flagMap))
			}
		} else {
			op.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
				return e.Create(entityBody(flagMap))
			}
		}
		info.Operations = append(info.Operations, op)
	}

	if e.UpdateWithContext != nil || e.Update != nil {
		op := EntityOperation{Verb: "update", ResponseType: responseTypeOf[R]()}
		if e.UpdateWithContext != nil {
			op.ContextDataFunc = func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
				id, err := entityIDFrom(flagMap, args)
				if err != nil {
					return nil, err
				}
				return e.UpdateWithContext(ctx, id, entityBodyExcludingID(flagMap))
			}
		} else {
			op.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
				id, err := entityIDFrom(flagMap, args)
				if err != nil {
					return nil, err
				}
				return e.Update(id, entityBodyExcludingID(flagMap))
			}
		}
		info.Operations = append(info.Operations, op)
	}

	if e.DeleteWithContext != nil || e.Delete != nil {
		op := EntityOperation{Verb: "delete"}
		if e.DeleteWithContext != nil {
			op.ContextDataFunc = func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
				id, err := entityIDFrom(flagMap, args)
				if err != nil {
					return nil, err
				}
				return nil, e.DeleteWithContext(ctx, id)
			}
		} else {
			op.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
				id, err := entityIDFrom(flagMap, args)
				if err != nil {
					return nil, err
				}
				return nil, e.Delete(id)
			}
		}
		info.Operations = append(info.Operations, op)
	}

	for _, action := range e.Actions {
		if action == nil {
			continue
		}
		info.Actions = append(info.Actions, action.actionInfo())
	}

	listType := reflect.TypeOf((*ListOpts)(nil)).Elem()
	for _, ba := range e.BulkActions {
		if ba == nil {
			continue
		}
		bai := ba.bulkActionInfo(func(flagMap map[string]string) (any, error) {
			return resolveEntityOpts[ListOpts](flagMap, e.Filters)
		})
		if bai.ListType == nil {
			bai.ListType = listType
		}
		if bai.FilterFunc != nil {
			bai.LookupFunc = buildLookupFunc[ListOpts](e.Filters)
			bai.ContextLookupFunc = buildLookupFuncWithContext[ListOpts](e.Filters)
			bai.BindCompletions = buildFilterCompletionBinder[ListOpts](e.Filters)
		}
		info.BulkActions = append(info.BulkActions, bai)
	}

	if e.Admin != nil {
		admin := *e.Admin
		if admin.Name == "" || admin.Name == name {
			admin.Name = name
		}
		adminValidArgs := admin.ValidArgs
		if adminValidArgs == nil {
			adminValidArgs = e.ValidArgs
		}
		// Store as admin sub-entity — GenerateCLI nests under "admin" parent
		adminInfo := EntityInfo{
			Name:      admin.Name,
			Type:      info.Type,
			ListType:  info.ListType,
			ValidArgs: adminValidArgs,
			IsAdmin:   true,
		}
		if admin.List != nil {
			adminInfo.Operations = append(adminInfo.Operations, EntityOperation{
				Verb: "list",
				DataFunc: func(flagMap map[string]string, args []string) (any, error) {
					opts, err := resolveEntityOpts[ListOpts](flagMap, admin.Filters)
					if err != nil {
						return nil, err
					}
					items, err := admin.List(opts)
					if err != nil {
						return nil, err
					}
					return withEntityIDs(items), nil
				},
				LookupFunc:        buildLookupFunc[ListOpts](admin.Filters),
				ContextLookupFunc: buildLookupFuncWithContext[ListOpts](admin.Filters),
				BindCompletions:   buildFilterCompletionBinder[ListOpts](admin.Filters),
				ResponseType:      reflect.TypeOf((*T)(nil)).Elem(),
				ResponseArray:     true,
				ResponseEntityID:  true,
			})
		}
		if admin.GetWithFlags != nil {
			adminInfo.Operations = append(adminInfo.Operations, EntityOperation{
				Verb:         "get",
				FlagsType:    actionFlagsType(admin.GetFlags),
				ResponseType: responseTypeOf[R](),
				DataFunc: func(flagMap map[string]string, args []string) (any, error) {
					id := flagMap["id"]
					if id == "" && len(args) > 0 {
						id = args[0]
					}
					if id == "" {
						return nil, fmt.Errorf("id is required")
					}
					return admin.GetWithFlags(id, flagMap)
				},
			})
		} else if admin.Get != nil {
			adminInfo.Operations = append(adminInfo.Operations, EntityOperation{
				Verb:         "get",
				ResponseType: responseTypeOf[R](),
				DataFunc: func(flagMap map[string]string, args []string) (any, error) {
					id := flagMap["id"]
					if id == "" && len(args) > 0 {
						id = args[0]
					}
					if id == "" {
						return nil, fmt.Errorf("id is required")
					}
					return admin.Get(id)
				},
			})
		}
		for _, action := range admin.Actions {
			if action == nil {
				continue
			}
			adminInfo.Actions = append(adminInfo.Actions, action.actionInfo())
		}
		entityRegistryMu.Lock()
		entityRegistry = append(entityRegistry, adminInfo)
		entityRegistryMu.Unlock()
	}

	entityRegistryMu.Lock()
	entityRegistry = append(entityRegistry, info)
	entityRegistryMu.Unlock()
}

// GetEntities returns all registered entities.
func GetEntities() []EntityInfo {
	entityRegistryMu.Lock()
	defer entityRegistryMu.Unlock()
	return append([]EntityInfo{}, entityRegistry...)
}

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
	entityCmd := &cobra.Command{
		Use:     entity.Name,
		Aliases: entity.Aliases,
		Short:   fmt.Sprintf("Manage %s resources", entity.Name),
	}
	annotateEntityCommand(entityCmd, entity)
	parent.AddCommand(entityCmd)

	for _, op := range entity.Operations {
		generateEntitySubcommand(entityCmd, entity, op)
	}

	for _, action := range entity.Actions {
		generateIDCommand(entityCmd, action.Name, action.Short, EntityOperation{
			Verb:            action.Name,
			Method:          action.Method,
			DataFunc:        action.DataFunc,
			ContextDataFunc: action.ContextDataFunc,
			FlagsType:       action.FlagsType,
			ResponseType:    action.ResponseType,
		}, entity.ValidArgs, "action", "", "entity", action.Name, "id", false, false, action.OptionalID)
	}

	for _, ba := range entity.BulkActions {
		generateBulkActionCommand(entityCmd, ba)
	}
}

func generateEntitySubcommand(parent *cobra.Command, entity EntityInfo, op EntityOperation) {
	switch op.Verb {
	case "list":
		generateListCommand(parent, entity, op)
	case "get":
		generateIDCommand(
			parent,
			"get",
			fmt.Sprintf("Get a %s by ID", entity.Name),
			op,
			entity.ValidArgs,
			"get",
			"",
			"entity",
			"",
			"id",
			false,
			false,
			false,
		)
	case "create":
		generateBodyCommand(parent, "create", fmt.Sprintf("Create a %s", entity.Name), op)
	case "update":
		generateBodyCommand(parent, "update", fmt.Sprintf("Update a %s", entity.Name), op)
	case "delete":
		generateIDCommand(
			parent,
			"delete",
			fmt.Sprintf("Delete a %s", entity.Name),
			op,
			entity.ValidArgs,
			"delete",
			"",
			"entity",
			"",
			"id",
			false,
			false,
			false,
		)
	}
}

// runEntityOp invokes the operation's data function on the CLI path, preferring
// the context-aware variant (fed cmd.Context()) when present.
func runEntityOp(c *cobra.Command, op EntityOperation, flagMap map[string]string, args []string) (any, error) {
	if op.ContextDataFunc != nil {
		return op.ContextDataFunc(c.Context(), flagMap, args)
	}
	return op.DataFunc(flagMap, args)
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
			MustPrint(result, Flags.FormatOptions)
			return nil
		},
	}

	// Bind filter flags from the ListOpts type
	bindTypeFlags(cmd, entity.ListType)
	if op.BindCompletions != nil {
		op.BindCompletions(cmd)
	}

	annotateEntityOperationCommand(cmd, parent, "list", "", "collection", "", "", op.LookupFunc != nil, false, false)
	parent.AddCommand(cmd)
	storeEntityDataFuncs(cmd, op)
	SetCommandResponseMeta(cmd, ResponseOpenAPIMeta{
		Type:     op.ResponseType,
		Array:    op.ResponseArray,
		EntityID: op.ResponseEntityID,
	})
	if op.LookupFunc != nil {
		lookupFuncRegistry.Store(cmd, op.LookupFunc)
	}
	if op.ContextLookupFunc != nil {
		contextLookupFuncRegistry.Store(cmd, op.ContextLookupFunc)
	}
}

func generateIDCommand(
	parent *cobra.Command,
	verb string,
	short string,
	op EntityOperation,
	validArgs func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective),
	metaVerb string,
	method string,
	scope string,
	actionName string,
	idParam string,
	supportsLookup bool,
	supportsFilterMode bool,
	optionalID bool,
) {
	hasFlags := op.FlagsType != nil
	idToken := "<id>"
	args := cobra.ExactArgs(1)
	if optionalID {
		idToken = "[id]"
		args = cobra.MaximumNArgs(1)
	}
	use := fmt.Sprintf("%s %s", verb, idToken)
	if hasFlags {
		use = fmt.Sprintf("%s %s [flags]", verb, idToken)
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
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
				return err
			}
			if result != nil {
				MustPrint(result, Flags.FormatOptions)
			}
			return nil
		},
	}
	if verb == "get" {
		cmd.Aliases = []string{"inspect"}
	}
	if validArgs != nil {
		cmd.ValidArgsFunction = validArgs
	}
	if hasFlags {
		bindTypeFlags(cmd, op.FlagsType)
	}
	if method == "" {
		method = op.Method
	}
	annotateEntityOperationCommand(cmd, parent, metaVerb, method, scope, actionName, idParam, supportsLookup, supportsFilterMode, optionalID)
	parent.AddCommand(cmd)
	storeEntityDataFuncs(cmd, op)
	SetCommandResponseMeta(cmd, ResponseOpenAPIMeta{
		Type:     op.ResponseType,
		Array:    op.ResponseArray,
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
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func generateBodyCommand(parent *cobra.Command, verb, short string, op EntityOperation) {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [key=value ...]", verb),
		Short: short,
		Args:  cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, args []string) error {
			flagMap := make(map[string]string)
			c.Flags().Visit(func(f *pflag.Flag) {
				flagMap[f.Name] = flagMapValue(f)
			})

			// For update, first arg is ID
			callArgs := args
			if verb == "update" && len(args) > 0 {
				flagMap["id"] = args[0]
				callArgs = args[1:]
			}

			if len(callArgs) > 0 {
				parsed, err := ParseArgumentsAsMap(callArgs)
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

			result, err := runEntityOp(c, op, flagMap, args)
			if err != nil {
				return err
			}
			if result != nil {
				MustPrint(result, Flags.FormatOptions)
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
	annotateEntityOperationCommand(cmd, parent, verb, "", scope, "", idParam, false, false, false)
	parent.AddCommand(cmd)
	storeEntityDataFuncs(cmd, op)
	SetCommandResponseMeta(cmd, ResponseOpenAPIMeta{
		Type:     op.ResponseType,
		Array:    op.ResponseArray,
		EntityID: op.ResponseEntityID,
	})
}

func generateBulkActionCommand(parent *cobra.Command, ba BulkActionInfo) {
	execute := func(flagMap map[string]string, args []string) (any, error) {
		if ba.FilterFunc != nil && flagMap["filter"] != "" {
			return ba.FilterFunc(flagMap, args)
		}
		return ba.DataFunc(flagMap, args)
	}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s <id> [id...]", ba.Name),
		Short: ba.Short,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			flagMap := make(map[string]string)
			c.Flags().Visit(func(f *pflag.Flag) {
				flagMap[f.Name] = flagMapValue(f)
			})

			// Use filter mode if --filter flag is set and FilterFunc exists
			result, err := execute(flagMap, args)
			if err != nil {
				return err
			}
			if result != nil {
				MustPrint(result, Flags.FormatOptions)
			}
			return nil
		},
	}

	if ba.FilterFunc != nil {
		bindTypeFlags(cmd, ba.ListType)
		if ba.BindCompletions != nil {
			ba.BindCompletions(cmd)
		}
	}

	annotateEntityOperationCommand(cmd, parent, "action", "", "collection", ba.Name, "id", ba.LookupFunc != nil, ba.FilterFunc != nil, false)
	parent.AddCommand(cmd)
	dataFuncRegistry.Store(cmd, execute)
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
	fieldInfos, _ := flags.ParseStructFields(t)
	for _, info := range fieldInfos {
		flags.BindFlag(cmd, info)
	}
}

func buildFilterCompletionBinder[T any](filters []Filter[T]) func(cmd *cobra.Command) {
	if len(filters) == 0 {
		return nil
	}

	return func(cmd *cobra.Command) {
		for _, filter := range filters {
			if cmd.Flag(filter.Key()) == nil {
				continue
			}
			if _, exists := cmd.GetFlagCompletionFunc(filter.Key()); exists {
				continue
			}

			filter := filter
			_ = cmd.RegisterFlagCompletionFunc(filter.Key(), func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
				flagMap := make(map[string]string)
				cmd.Flags().Visit(func(f *pflag.Flag) {
					if f.Name == filter.Key() {
						return
					}
					flagMap[f.Name] = flagMapValue(f)
				})

				opts, err := resolveEntityOpts[T](flagMap, filters)
				if err != nil {
					return nil, cobra.ShellCompDirectiveError
				}

				options := filter.Options(opts)
				if len(options) == 0 {
					return nil, cobra.ShellCompDirectiveNoFileComp
				}

				keys := make([]string, 0, len(options))
				for key := range options {
					if strings.HasPrefix(key, toComplete) {
						keys = append(keys, key)
					}
				}
				sort.Strings(keys)

				completions := make([]string, 0, len(keys))
				for _, key := range keys {
					description := sanitizeCompletionDescription(options[key].String())
					if description == "" || description == key {
						completions = append(completions, key)
						continue
					}
					completions = append(completions, cobra.CompletionWithDesc(key, description))
				}

				return completions, cobra.ShellCompDirectiveNoFileComp
			})
		}
	}
}

func sanitizeCompletionDescription(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\t", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

// buildOpts creates an instance of T and populates it from a flag map.
func buildOpts[T any](flagMap map[string]string) (T, error) {
	var zero T
	t := reflect.TypeOf((*T)(nil)).Elem()
	if t.Kind() != reflect.Struct {
		return zero, fmt.Errorf("expected struct options, got %s", t.Kind())
	}

	cmd := &cobra.Command{Use: "opts"}
	fieldInfos, err := flags.ParseStructFields(t)
	if err != nil {
		return zero, err
	}
	flagValues := make([]*flags.FlagValue, 0, len(fieldInfos))
	for _, info := range fieldInfos {
		if fv := flags.BindFlag(cmd, info); fv != nil {
			flagValues = append(flagValues, fv)
		}
	}

	for name, val := range flagMap {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			continue
		}
		if err := flag.Value.Set(val); err != nil {
			return zero, fmt.Errorf("setting flag %s: %w", name, err)
		}
		flag.Changed = true
	}

	v := reflect.New(t).Elem()
	for _, fv := range flagValues {
		if err := flags.AssignFieldValue(v, fv, nil, false); err != nil {
			return zero, err
		}
	}

	return v.Interface().(T), nil
}

func resolveEntityOpts[T any](flagMap map[string]string, filters []Filter[T]) (T, error) {
	opts, err := buildOpts[T](flagMap)
	if err != nil {
		return opts, err
	}
	_, err = applyEntityFilters(&opts, filters)
	return opts, err
}

// lookupOptionsLimit caps the number of options a single lookup filter returns,
// both for the head set and per-search. SearchableFilter implementations enumerate
// only this many; the UI shows "… and N more" against the reported total and
// re-queries as the user types.
const lookupOptionsLimit = 200

// Reserved flag keys the lookup handler injects from the request query so a
// single filter can be searched server-side. They never collide with entity
// flags because of the __lookup_ prefix.
const (
	lookupFilterKeyParam = "__lookup_filter"
	lookupQueryParam     = "__lookup_q"
)

func buildLookupFunc[T any](filters []Filter[T]) func(flags map[string]string, args []string) (any, error) {
	if len(filters) == 0 {
		return nil
	}
	lookupMetadata := buildLookupMetadata[T]()
	return func(flagMap map[string]string, args []string) (any, error) {
		return resolveLookup(context.Background(), filters, lookupMetadata, flagMap)
	}
}

// buildLookupFuncWithContext is the context-aware variant of buildLookupFunc.
// The closure threads the request ctx into each filter, so a filter that
// implements ContextFilter/ContextSearchableFilter can resolve request-scoped
// state (e.g. the per-request database handle). Filters that don't implement
// the context-aware interfaces behave exactly as under buildLookupFunc.
func buildLookupFuncWithContext[T any](filters []Filter[T]) func(ctx context.Context, flags map[string]string, args []string) (any, error) {
	if len(filters) == 0 {
		return nil
	}
	lookupMetadata := buildLookupMetadata[T]()
	return func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
		return resolveLookup(ctx, filters, lookupMetadata, flagMap)
	}
}

// resolveLookup builds the entity lookup response for one request. It strips
// the reserved search params, resolves opts + selected values, then fills each
// filter's option set — preferring the context-aware Options methods when the
// filter implements them, otherwise the plain Options/OptionsWithQuery.
func resolveLookup[T any](
	ctx context.Context,
	filters []Filter[T],
	lookupMetadata map[string]entityLookupMetadata,
	flagMap map[string]string,
) (any, error) {
	searchKey := flagMap[lookupFilterKeyParam]
	searchQuery := flagMap[lookupQueryParam]
	// The reserved params aren't entity flags; strip them before buildOpts
	// so they don't leak into the filter ListOpts.
	delete(flagMap, lookupFilterKeyParam)
	delete(flagMap, lookupQueryParam)

	opts, err := buildOpts[T](flagMap)
	if err != nil {
		return nil, err
	}

	selected, err := applyEntityFilters(&opts, filters)
	if err != nil {
		return nil, err
	}

	response := entityLookupResponse{
		Filters: make(map[string]entityLookupFilter, len(filters)),
	}
	for _, filter := range filters {
		meta := lookupMetadata[filter.Key()]
		entry := entityLookupFilter{
			Label:    filter.Label(),
			Selected: toClickyNodeMap(selected[filter.Key()]),
			Multi:    meta.Multi,
			Type:     meta.Type,
		}

		isSearchable := filterIsSearchable(filter)
		switch {
		case isSearchable && searchKey == filter.Key() && searchQuery != "":
			// Targeted server-side search: return matches for this filter only.
			options, _ := filterOptionsWithQuery(ctx, filter, opts, searchQuery, lookupOptionsLimit)
			entry.Options = toClickyNodeMap(options)
		case isSearchable:
			// Head request: first N options plus the true distinct total so
			// the UI can show "… and N more" and decide whether to search.
			options, total := filterOptionsWithQuery(ctx, filter, opts, "", lookupOptionsLimit)
			entry.Options = toClickyNodeMap(options)
			entry.Total = total
			entry.Truncated = total > len(options)
		default:
			entry.Options = toClickyNodeMap(filterOptions(ctx, filter, opts))
		}

		response.Filters[filter.Key()] = entry
	}
	return response, nil
}

// filterIsSearchable reports whether filter exposes a head/search option set,
// via either the plain or the context-aware searchable interface.
func filterIsSearchable[T any](filter Filter[T]) bool {
	if _, ok := filter.(SearchableFilter[T]); ok {
		return true
	}
	_, ok := filter.(ContextSearchableFilter[T])
	return ok
}

// filterOptions resolves a filter's option map, preferring its context-aware
// OptionsWithContext when implemented so it can reach request-scoped state.
func filterOptions[T any](ctx context.Context, filter Filter[T], opts T) map[string]api.Textable {
	if c, ok := filter.(ContextFilter[T]); ok {
		return c.OptionsWithContext(ctx, opts)
	}
	return filter.Options(opts)
}

// filterOptionsWithQuery resolves a searchable filter's head/search results,
// preferring the context-aware interface, then the plain searchable one.
func filterOptionsWithQuery[T any](ctx context.Context, filter Filter[T], opts T, query string, limit int) (map[string]api.Textable, int) {
	if c, ok := filter.(ContextSearchableFilter[T]); ok {
		return c.OptionsWithQueryAndContext(ctx, opts, query, limit)
	}
	if s, ok := filter.(SearchableFilter[T]); ok {
		return s.OptionsWithQuery(opts, query, limit)
	}
	options := filterOptions(ctx, filter, opts)
	return options, len(options)
}

type entityLookupMetadata struct {
	Multi bool
	Type  string
}

func buildLookupMetadata[T any]() map[string]entityLookupMetadata {
	structType := reflect.TypeOf((*T)(nil)).Elem()
	if structType.Kind() != reflect.Struct {
		return nil
	}

	fields, err := flags.ParseStructFields(structType)
	if err != nil {
		return nil
	}

	metadata := make(map[string]entityLookupMetadata, len(fields))
	for _, field := range fields {
		if field.FlagName == "" || field.FlagName == "-" {
			continue
		}
		metadata[field.FlagName] = describeLookupField(field)
	}

	return metadata
}

func describeLookupField(field flags.FieldInfo) entityLookupMetadata {
	fieldType := field.FieldType
	for fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	meta := entityLookupMetadata{
		Multi: fieldType.Kind() == reflect.Slice,
	}

	switch {
	case fieldType == multiFilterType:
		meta.Type = "multi-filter"
	case fieldType.Kind() == reflect.Bool:
		meta.Type = "bool"
	case isNumericKind(fieldType.Kind()):
		meta.Type = "number"
	case fieldType.String() == "time.Time":
		switch {
		case isRangeStartFlag(field.FlagName):
			meta.Type = "from"
		case isRangeEndFlag(field.FlagName):
			meta.Type = "to"
		default:
			meta.Type = "date"
		}
	}

	return meta
}

func isNumericKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func isRangeStartFlag(name string) bool {
	return name == "from" || strings.HasSuffix(name, "-from")
}

func isRangeEndFlag(name string) bool {
	return name == "to" || strings.HasSuffix(name, "-to")
}

func applyEntityFilters[T any](opts *T, filters []Filter[T]) (map[string]map[string]api.Textable, error) {
	selected := make(map[string]map[string]api.Textable, len(filters))
	for _, filter := range filters {
		values, err := filter.Lookup(opts)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filter.Key(), err)
		}
		if len(values) > 0 {
			selected[filter.Key()] = values
		}
	}
	return selected, nil
}

// withEntityIDs wraps each EntityItem in the slice to include a _id field in JSON output.
func withEntityIDs[T EntityItem](items []T) []entityWithID[T] {
	result := make([]entityWithID[T], len(items))
	for i, item := range items {
		result[i] = entityWithID[T]{ID: item.GetID(), Inner: item}
	}
	return result
}

type entityWithID[T EntityItem] struct {
	ID    string `json:"_id"`
	Inner T
}

func (e entityWithID[T]) GetID() string   { return e.ID }
func (e entityWithID[T]) GetName() string { return e.Inner.GetName() }

func (e entityWithID[T]) Columns() []api.ColumnDef {
	withID := func(columns []api.ColumnDef) []api.ColumnDef {
		for _, column := range columns {
			if column.Name == "_id" {
				return columns
			}
		}
		return append([]api.ColumnDef{api.Column("_id").Hidden().Build()}, columns...)
	}

	if tp, ok := any(e.Inner).(api.TableProvider); ok {
		return withID(tp.Columns())
	}

	if prettyRow, ok := entityPrettyRow(any(e.Inner), nil); ok {
		columns := columnsFromPrettyRow(prettyRow)
		if len(columns) > 0 {
			return withID(columns)
		}
	}

	columns, _, ok := columnsAndRowFromStruct(any(e.Inner))
	if !ok {
		return nil
	}
	return withID(columns)
}

func (e entityWithID[T]) Row() map[string]any {
	withID := func(row map[string]any) map[string]any {
		next := make(map[string]any, len(row)+1)
		next["_id"] = e.ID
		for key, value := range row {
			next[key] = value
		}
		return next
	}

	if tp, ok := any(e.Inner).(api.TableProvider); ok {
		return withID(tp.Row())
	}

	if prettyRow, ok := entityPrettyRow(any(e.Inner), nil); ok {
		row := make(map[string]any, len(prettyRow))
		for key, value := range prettyRow {
			row[key] = value
		}
		return withID(row)
	}

	_, row, ok := columnsAndRowFromStruct(any(e.Inner))
	if !ok {
		return nil
	}
	return withID(row)
}

func (e entityWithID[T]) PrettyRow(opts interface{}) map[string]api.Text {
	if row, ok := entityPrettyRow(any(e.Inner), opts); ok {
		next := make(map[string]api.Text, len(row)+1)
		next["_id"] = api.Text{Content: e.ID}
		for key, value := range row {
			next[key] = value
		}
		return next
	}

	rowData := e.Row()
	if len(rowData) == 0 {
		return nil
	}

	row := make(map[string]api.Text)
	order := 1
	for _, col := range e.Columns() {
		if col.Hidden {
			continue
		}
		cell := api.Text{Style: fmt.Sprintf("order-%d %s", order, col.Style)}
		if value, ok := rowData[col.Name]; ok {
			cell = addPrettyRowValue(cell, value)
		}
		row[col.DisplayLabel()] = cell
		order++
	}
	return row
}

func entityPrettyRow(inner any, opts interface{}) (map[string]api.Text, bool) {
	if pr, ok := inner.(api.PrettyRow); ok {
		if row := pr.PrettyRow(opts); len(row) > 0 {
			return row, true
		}
	}
	return nil, false
}

func columnsFromPrettyRow(prettyRow map[string]api.Text) []api.ColumnDef {
	type orderedColumn struct {
		name  string
		style string
		order int
	}

	columns := make([]orderedColumn, 0, len(prettyRow))
	for name, text := range prettyRow {
		columns = append(columns, orderedColumn{
			name:  name,
			style: text.Style,
			order: api.ExtractOrderValue(text.Style),
		})
	}

	sort.Slice(columns, func(i, j int) bool {
		if columns[i].order != columns[j].order {
			return columns[i].order < columns[j].order
		}
		return columns[i].name < columns[j].name
	})

	defs := make([]api.ColumnDef, 0, len(columns))
	for _, col := range columns {
		defs = append(defs, api.ColumnDef{
			Name:  col.name,
			Label: col.name,
			Style: col.style,
		})
	}
	return defs
}

func columnsAndRowFromStruct(inner any) ([]api.ColumnDef, map[string]any, bool) {
	parser := api.NewStructParser()
	val := reflect.ValueOf(inner)
	if !val.IsValid() {
		return nil, nil, false
	}

	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, nil, false
		}
		val = val.Elem()
	}

	fields, err := parser.GetTableFields(val)
	if err != nil {
		return nil, nil, false
	}

	typedRow, err := parser.StructToRow(val)
	if err != nil {
		return nil, nil, false
	}

	columns := make([]api.ColumnDef, 0, len(fields))
	row := make(map[string]any, len(fields))
	for _, field := range fields {
		columns = append(columns, api.ColumnDef{
			Name:          field.Name,
			Label:         field.Label,
			Style:         field.Style,
			HeaderStyle:   field.LabelStyle,
			Type:          field.Type,
			Format:        field.Format,
			FormatOptions: field.FormatOptions,
		})
		if value, ok := typedRow[field.Name]; ok {
			row[field.Name] = value.Value()
		}
	}

	return columns, row, true
}

func addPrettyRowValue(cell api.Text, value any) api.Text {
	switch v := value.(type) {
	case api.PrettyShort:
		return cell.Add(v.PrettyShort())
	case api.Textable:
		return cell.Add(v)
	case api.Pretty:
		return cell.Add(v.Pretty())
	default:
		return cell.Add(api.Human(v))
	}
}

func (e entityWithID[T]) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(e.Inner)
	if err != nil {
		return nil, err
	}
	if len(data) < 2 || data[0] != '{' {
		return data, nil
	}
	idJSON, err := json.Marshal(e.ID)
	if err != nil {
		return nil, err
	}
	prefix := []byte(`{"_id":`)
	prefix = append(prefix, idJSON...)
	prefix = append(prefix, ',')
	return append(prefix, data[1:]...), nil
}
