package entity

import (
	"context"
	"encoding/csv"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/flanksource/clicky/api"
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
	Name    string
	Parent  string
	Aliases []string
	// Icon is an opaque UI icon name carried through to the OpenAPI surface
	// (x-clicky.surfaces[].icon). Empty for entities that declare no icon.
	Icon string
	// Path is the entity's hierarchy position within Parent, PathSeparator-joined
	// (x-clicky.surfaces[].path). Empty for entities that declare no hierarchy.
	Path string
	// Title overrides the auto-generated surface title when non-empty.
	Title         string
	Type          reflect.Type
	ListType      reflect.Type
	Operations    []EntityOperation
	PrimaryAction *ActionInfo
	Actions       []ActionInfo
	BulkActions   []BulkActionInfo
	ValidArgs     func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)
	// IDOptions lists the ids the <id> argument offers, with their names and
	// descriptions (nil when the entity declares its own ValidArgs).
	IDOptions IDOptionsFunc
	IsAdmin   bool
	// ServedOnly keeps the generated commands off the CLI; see Entity.ServedOnly.
	ServedOnly bool
	// FilterRefs maps a list filter's flag key to the name of the reusable named
	// filter backing it (when the filter was attached via the entity package's
	// Use/As helper). The OpenAPI layer uses it to emit an x-clicky-lookup $ref
	// to the shared filter definition. Nil when no filters reference a named one.
	FilterRefs map[string]string
	// ToolGroup names the group all of this entity's operations belong to. It is
	// inherited by every generated operation (an action may override it) and is
	// used by AI tool-preference layers to toggle related tools together.
	ToolGroup string
	ToolHints MCPToolHints
}

// EntityOperation represents a single CRUD operation.
type EntityOperation struct {
	Verb   string // "list", "get", "create", "update", "delete"
	Method string // Optional explicit HTTP method for generated RPC/OpenAPI routes.
	// RoutePath publishes the operation at a declared URL instead of the one
	// its command tree implies. An absolute path is used verbatim; anything
	// else is relative to the configured prefix. Empty keeps the derived path.
	RoutePath string
	DataFunc  func(flags map[string]string, args []string) (any, error)
	// ContextDataFunc, when set, is preferred over DataFunc by both the CLI run
	// path (fed cmd.Context()) and the RPC executor (fed r.Context()). Lets the
	// operation resolve request-scoped state instead of process globals.
	ContextDataFunc ContextDataFunc
	// FlagsType, when non-nil, binds typed flags from the given struct type
	// onto the generated cobra command and collects their values into the
	// flag map passed to DataFunc. Used by actions that implement
	// ActionFlags and by the get and delete verbs when the entity declares
	// GetFlags or DeleteFlags.
	FlagsType  reflect.Type
	LookupFunc func(flags map[string]string, args []string) (any, error)
	// ContextLookupFunc, when set, is the context-aware filter lookup closure.
	// The RPC converter wires it onto RPCOperation.ContextLookupFunc and the
	// executor prefers it over LookupFunc, feeding the request context.
	ContextLookupFunc ContextLookupFunc
	BindCompletions   func(cmd *cobra.Command)
	// FlagSchemas shape JSON-object flags from the other arguments.
	FlagSchemas      map[string]FlagSchema
	ResponseType     reflect.Type
	ResponseArray    bool
	ResponsePaged    bool
	ResponseEntityID bool
	Sort             *SortSpec
	Schedule         *OperationScheduleMeta
}

// ActionInfo is the type-erased representation of a single-entity action.
type ActionInfo struct {
	Name   string
	Short  string
	Method string
	// RoutePath publishes the action at a declared URL rather than the derived
	// one. See ActionSpec.WithPath.
	RoutePath string
	DataFunc  func(flags map[string]string, args []string) (any, error)
	// ContextDataFunc, when set, is preferred over DataFunc by both the CLI run
	// path (fed cmd.Context()) and the RPC executor (fed r.Context()), mirroring
	// the entity CRUD seam. Lets an action resolve request-scoped state instead
	// of process globals.
	ContextDataFunc   ContextDataFunc
	LookupFunc        func(flags map[string]string, args []string) (any, error)
	ContextLookupFunc ContextLookupFunc
	BindCompletions   func(cmd *cobra.Command)
	// FlagSchemas shape JSON-object flags from the other arguments
	// (WithActionFlagSchema).
	FlagSchemas map[string]FlagSchema
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
	// ToolGroup overrides the entity's tool group for this action only. Empty
	// means the action inherits the entity's group.
	ToolGroup string
	ToolHints MCPToolHints
	Schedule  *OperationScheduleMeta
}

// BulkActionInfo is the type-erased representation of a bulk action.
type BulkActionInfo struct {
	Name              string
	Short             string
	DataFunc          func(flags map[string]string, args []string) (any, error)
	ContextDataFunc   ContextDataFunc
	FilterFunc        func(flags map[string]string, args []string) (any, error)
	ContextFilterFunc ContextDataFunc
	LookupFunc        func(flags map[string]string, args []string) (any, error)
	ContextLookupFunc ContextLookupFunc
	ListType          reflect.Type
	BindCompletions   func(cmd *cobra.Command)
	ResponseType      reflect.Type
	// FlagsType, if non-nil, is the struct type whose `flag:"..."` tagged fields
	// are the action's own parameters — bound on the generated command and
	// published as OpenAPI parameters, exactly as ActionInfo.FlagsType is.
	FlagsType reflect.Type
	// ToolGroup overrides the entity's tool group for this action only. Empty
	// means the action inherits the entity's group.
	ToolGroup string
	ToolHints MCPToolHints
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

// TypedFilter is an OPTIONAL extension a Filter may implement when the
// underlying flag type is intentionally broader than the UI control type. For
// example, date-math flags often remain strings at the CLI layer while the web
// lookup response should still advertise a from/to date-range control.
type TypedFilter[ListOpts any] interface {
	LookupType() string
}

// UnitFilter is an OPTIONAL extension a Filter may implement when the numeric
// value shown by its control has a declared unit. It is unparameterized because
// the unit is presentation metadata, independent of the options type.
type UnitFilter interface {
	LookupUnit() string
}

type DefaultOperatorFilter interface {
	LookupDefaultOperator() string
}

// LimitedFilter is an OPTIONAL extension a Filter may implement to cap its own
// option set below the package ceiling. It is unparameterized because the cap is
// a property of the filter's cardinality, not of the entity it is attached to.
type LimitedFilter interface {
	// LookupLimit returns the head/search cap, or zero to take the default.
	LookupLimit() int
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

// FallibleFilter is an OPTIONAL extension for a filter whose options come
// from a source that can fail (a database, a config catalog). Lookups and
// shell completion prefer it over Options/OptionsWithContext, and its error
// fails the lookup instead of leaving an empty option set.
type FallibleFilter[ListOpts any] interface {
	LookupOptions(ctx context.Context, opts ListOpts) (map[string]api.Textable, error)
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
func (l liftedFilter[Outer, Inner]) LookupType() string {
	if typed, ok := l.inner.(TypedFilter[Inner]); ok {
		return typed.LookupType()
	}
	return ""
}

func (l liftedFilter[Outer, Inner]) LookupUnit() string {
	if unit, ok := l.inner.(UnitFilter); ok {
		return unit.LookupUnit()
	}
	return ""
}

func (l liftedFilter[Outer, Inner]) LookupDefaultOperator() string {
	if filter, ok := l.inner.(DefaultOperatorFilter); ok {
		return filter.LookupDefaultOperator()
	}
	return ""
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

func (l liftedFilter[Outer, Inner]) lookupOptions(ctx context.Context, opts Outer, query string, limit int) (FilterOptions, error) {
	return typedFilterOptions(ctx, l.inner, filterOptionsRequest[Inner]{
		Opts:       *l.project(&opts),
		Searchable: filterIsSearchable(l.inner),
		Query:      query,
		Limit:      limit,
	})
}

// EntityAction is the type-erased registration surface for custom entity
// actions. Use Action or ActionWithFlags to construct values.
type EntityAction interface {
	actionInfo() ActionInfo
}

type ActionSpec[R any] struct {
	name              string
	short             string
	method            string
	routePath         string
	run               func(id string, flags map[string]string) (R, error)
	runCtx            func(ctx context.Context, id string, flags map[string]string) (R, error)
	flags             ActionFlags
	lookupFunc        DataFunc
	contextLookupFunc ContextLookupFunc
	bindCompletions   func(cmd *cobra.Command)
	flagSchemas       map[string]FlagSchema
	optionalID        bool
	toolGroup         string
	toolHints         MCPToolHints
	schedule          *OperationScheduleMeta
}

// dataOrError type-erases a typed handler result. When the handler failed, a
// zero R (including a typed nil pointer or interface) is dropped: boxed into a
// non-nil `any`, callers that branch on `data != nil` (the RPC executor
// serializing partial results) would render a zero struct as the response body,
// hiding the error behind `{"field": "", ...}`. A non-zero result returned with
// an error is a partial result — e.g. a failed run carrying the trace of where it
// failed — and is kept so it reaches the caller alongside the error.
func dataOrError[R any](result R, err error) (any, error) {
	if err == nil {
		return result, nil
	}
	v := reflect.ValueOf(&result).Elem()
	if v.IsZero() || (v.Kind() == reflect.Interface && v.Elem().Kind() == reflect.Pointer && v.Elem().IsNil()) {
		return nil, err
	}
	return result, err
}

// Action creates a typed custom operation on a single entity by ID.
func Action[R any](name string, fn func(id string, flags map[string]string) (R, error)) *ActionSpec[R] {
	return &ActionSpec[R]{name: name, run: fn}
}

// ActionWithFlags creates a typed custom operation with typed action flags.
func ActionWithFlags[R any](name string, flags ActionFlags, fn func(id string, flags map[string]string) (R, error)) *ActionSpec[R] {
	return &ActionSpec[R]{name: name, flags: flags, run: fn}
}

// ActionWithContext creates a typed action whose run closure receives the
// request-scoped context (cmd.Context() on the CLI, r.Context() over HTTP), so
// it can resolve a per-request database/config instead of a process global.
func ActionWithContext[R any](name string, fn func(ctx context.Context, id string, flags map[string]string) (R, error)) *ActionSpec[R] {
	return &ActionSpec[R]{name: name, runCtx: fn}
}

// ActionWithFlagsAndContext is ActionWithContext with typed action flags.
func ActionWithFlagsAndContext[R any](name string, flags ActionFlags, fn func(ctx context.Context, id string, flags map[string]string) (R, error)) *ActionSpec[R] {
	return &ActionSpec[R]{name: name, flags: flags, runCtx: fn}
}

// TypedActionWithContext is ActionWithFlagsAndContext with the flags already
// resolved into their options struct, the way PrimaryActionWithContext resolves
// them for the collection action. A handler that reads its flags out of the raw
// map has to repeat every flag name the struct already declares, and the two
// drift apart silently the first time one is renamed.
func TypedActionWithContext[Opts ActionFlags, R any](name string, opts Opts, fn func(ctx context.Context, id string, opts Opts) (R, error)) *ActionSpec[R] {
	var actionFilters []Filter[Opts]
	if filterable, ok := any(opts).(Filterable[Opts]); ok {
		actionFilters = filterable.Filters()
	}
	action := ActionWithFlagsAndContext(name, opts, func(ctx context.Context, id string, flagMap map[string]string) (R, error) {
		resolved, err := resolveActionOpts(ctx, id, flagMap, actionFilters)
		if err != nil {
			var zero R
			return zero, err
		}
		return fn(ctx, id, resolved)
	})
	return WithActionFilters(action, actionFilters)
}

func (a *ActionSpec[R]) WithShort(short string) *ActionSpec[R] {
	a.short = short
	return a
}

func (a *ActionSpec[R]) WithFlags(flags ActionFlags) *ActionSpec[R] {
	a.flags = flags
	return a
}

// WithMethod overrides the inferred HTTP method for the generated RPC/OpenAPI
// action route. Leave empty to keep the default inference behavior.
func (a *ActionSpec[R]) WithMethod(method string) *ActionSpec[R] {
	a.method = method
	return a
}

// WithPath publishes the action at a declared URL instead of the one its
// command tree implies. An absolute path is used verbatim; anything else is
// relative to the configured prefix. Leave empty to keep the derived path.
//
// Use it when callers already address the operation somewhere else — a route
// being converted from a hand-written handler, or a URL an earlier release
// established. The path may name wildcards the command does not, e.g.
// "/api/v1/cycle-runs/{id}/levels/{level}/cancel".
func (a *ActionSpec[R]) WithPath(path string) *ActionSpec[R] {
	a.routePath = path
	return a
}

// WithOptionalID makes the positional <id> argument optional on the
// generated action command. Use for actions whose target is supplied
// entirely through flags; the `id` passed to the run func is then empty.
func (a *ActionSpec[R]) WithOptionalID() *ActionSpec[R] {
	a.optionalID = true
	return a
}

// WithToolGroup overrides the entity's tool group for this action only. Empty
// (the default) means the action inherits the entity's group.
func (a *ActionSpec[R]) WithToolGroup(group string) *ActionSpec[R] {
	a.toolGroup = group
	a.toolHints.Group = group
	return a
}

// WithToolPermission sets this action's default approval mode. Actions carry no
// verb default: unset defers to the entity's ToolPermission or, failing that,
// the app's approval policy.
func (a *ActionSpec[R]) WithToolPermission(permission ToolPermission) *ActionSpec[R] {
	a.toolHints.DefaultPermission = permission
	return a
}

// WithToolHints overrides this action's inherited MCP tool hints.
func (a *ActionSpec[R]) WithToolHints(hints MCPToolHints) *ActionSpec[R] {
	a.toolHints = a.toolHints.merge(hints)
	if hints.Group != "" {
		a.toolGroup = hints.Group
	}
	return a
}

// WithSchedule opts this action into operation scheduling. Suggestions are
// shown in editors but do not create persisted schedules.
func (a *ActionSpec[R]) WithSchedule(meta OperationScheduleMeta) *ActionSpec[R] {
	copy := meta
	copy.Suggestions = append([]ScheduleSuggestion(nil), meta.Suggestions...)
	a.schedule = &copy
	return a
}

func (a *ActionSpec[R]) actionID(flagMap map[string]string, args []string) (string, error) {
	id := flagMap["id"]
	if id == "" && len(args) > 0 {
		id = args[0]
	}
	if id == "" && !a.optionalID {
		return "", fmt.Errorf("id is required")
	}
	return id, nil
}

func (a *ActionSpec[R]) actionInfo() ActionInfo {
	info := ActionInfo{
		Name:              a.name,
		Short:             a.short,
		Method:            a.method,
		RoutePath:         a.routePath,
		FlagsType:         actionFlagsType(a.flags),
		ResponseType:      responseTypeOf[R](),
		OptionalID:        a.optionalID,
		ToolGroup:         a.toolGroup,
		ToolHints:         a.toolHints,
		Schedule:          a.schedule,
		LookupFunc:        a.lookupFunc,
		ContextLookupFunc: a.contextLookupFunc,
		BindCompletions:   a.bindCompletions,
		FlagSchemas:       a.flagSchemas,
	}
	if a.runCtx != nil {
		info.ContextDataFunc = func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
			id, err := a.actionID(flagMap, args)
			if err != nil {
				return nil, err
			}
			return dataOrError(a.runCtx(ctx, id, flagMap))
		}
	} else {
		info.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
			id, err := a.actionID(flagMap, args)
			if err != nil {
				return nil, err
			}
			return dataOrError(a.run(id, flagMap))
		}
	}
	return info
}

// EntityBulkAction is the type-erased registration surface for custom bulk
// actions. Use BulkAction or BulkFilterAction to construct values.
type EntityBulkAction interface {
	bulkActionInfo(resolveOpts func(map[string]string) (any, error)) BulkActionInfo
}

type BulkActionSpec[R any] struct {
	name          string
	short         string
	run           func(ids []string, flags map[string]string) (R, error)
	runCtx        func(context.Context, []string, map[string]string) (R, error)
	filterFunc    func(opts any, flags map[string]string) (R, error)
	filterFuncCtx func(context.Context, any, map[string]string) (R, error)
	listType      reflect.Type
	// flags are the action's own parameters, declared the same way a single
	// action declares them. Without them a bulk action reaching id mode gets an
	// unbound flag map: whatever the caller happened to send, with no cobra
	// flags, no OpenAPI parameters and no validation. Filter mode inherits the
	// entity's ListOpts flags, which say how to *select* rows, never what to do
	// to them — so "set status to done on these forty" had nowhere to put "done".
	flags     ActionFlags
	toolGroup string
	toolHints MCPToolHints
}

// BulkAction creates a typed custom operation on multiple entity IDs.
func BulkAction[R any](name string, fn func(ids []string, flags map[string]string) (R, error)) *BulkActionSpec[R] {
	return &BulkActionSpec[R]{name: name, run: fn}
}

// BulkActionWithContext creates an ID-based bulk operation with request-scoped context.
func BulkActionWithContext[R any](name string, fn func(context.Context, []string, map[string]string) (R, error)) *BulkActionSpec[R] {
	return &BulkActionSpec[R]{name: name, runCtx: fn}
}

// BulkFilterAction creates a typed custom operation that runs against a typed
// filtered list selection instead of explicit IDs.
func BulkFilterAction[ListOpts any, R any](name string, fn func(opts ListOpts, flags map[string]string) (R, error)) *BulkActionSpec[R] {
	listType := reflect.TypeOf((*ListOpts)(nil)).Elem()
	return &BulkActionSpec[R]{
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
) *BulkActionSpec[R] {
	action := BulkFilterAction(name, runFilter)
	action.run = run
	return action
}

// BulkActionWithFilterAndContext creates a bulk action whose selected and
// filtered handlers receive the request-scoped context.
func BulkActionWithFilterAndContext[ListOpts any, R any](
	name string,
	run func(context.Context, []string, map[string]string) (R, error),
	runFilter func(context.Context, ListOpts, map[string]string) (R, error),
) *BulkActionSpec[R] {
	listType := reflect.TypeOf((*ListOpts)(nil)).Elem()
	return &BulkActionSpec[R]{
		name:     name,
		listType: listType,
		runCtx:   run,
		filterFuncCtx: func(ctx context.Context, opts any, flagMap map[string]string) (R, error) {
			typed, ok := opts.(ListOpts)
			if !ok {
				var zero R
				return zero, fmt.Errorf("expected %s options, got %T", listType, opts)
			}
			return runFilter(ctx, typed, flagMap)
		},
	}
}

func (b *BulkActionSpec[R]) WithShort(short string) *BulkActionSpec[R] {
	b.short = short
	return b
}

// WithFlags declares the action's own typed parameters, mirroring
// ActionSpec.WithFlags. They bind alongside the entity's ListOpts flags rather
// than instead of them: a bulk action is a selection *and* an operation, and the
// two halves are independent — `--status pending` picks the rows, `--to done`
// says what happens to them.
func (b *BulkActionSpec[R]) WithFlags(flags ActionFlags) *BulkActionSpec[R] {
	b.flags = flags
	return b
}

// WithToolGroup overrides the entity's tool group for this bulk action only.
func (b *BulkActionSpec[R]) WithToolGroup(group string) *BulkActionSpec[R] {
	b.toolGroup = group
	b.toolHints.Group = group
	return b
}

// WithToolPermission sets this bulk action's default approval mode. A bulk
// action is the one most worth asking about — it is the same verb aimed at
// forty rows instead of one.
func (b *BulkActionSpec[R]) WithToolPermission(permission ToolPermission) *BulkActionSpec[R] {
	b.toolHints.DefaultPermission = permission
	return b
}

// WithToolHints overrides this bulk action's inherited MCP tool hints. Icon,
// Group and DestructiveHint are also what a UI needs to render the action in a
// selection toolbar, so they are declared once here rather than duplicated in
// every front end.
func (b *BulkActionSpec[R]) WithToolHints(hints MCPToolHints) *BulkActionSpec[R] {
	b.toolHints = b.toolHints.merge(hints)
	if hints.Group != "" {
		b.toolGroup = hints.Group
	}
	return b
}

func (b *BulkActionSpec[R]) bulkActionInfo(resolveOpts func(map[string]string) (any, error)) BulkActionInfo {
	info := BulkActionInfo{
		Name:         b.name,
		Short:        b.short,
		ListType:     b.listType,
		ResponseType: responseTypeOf[R](),
		FlagsType:    actionFlagsType(b.flags),
		ToolGroup:    b.toolGroup,
		ToolHints:    b.toolHints,
	}
	if b.runCtx != nil {
		info.ContextDataFunc = func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
			return dataOrError(b.runCtx(ctx, args, flagMap))
		}
	} else if b.run != nil {
		info.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
			return dataOrError(b.run(args, flagMap))
		}
	} else {
		info.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
			return nil, fmt.Errorf("bulk action %q requires filter mode", b.name)
		}
	}
	if b.filterFuncCtx != nil {
		info.ContextFilterFunc = func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
			opts, err := resolveOpts(flagMap)
			if err != nil {
				return nil, err
			}
			return dataOrError(b.filterFuncCtx(ctx, opts, flagMap))
		}
	} else if b.filterFunc != nil {
		info.FilterFunc = func(flagMap map[string]string, args []string) (any, error) {
			opts, err := resolveOpts(flagMap)
			if err != nil {
				return nil, err
			}
			return dataOrError(b.filterFunc(opts, flagMap))
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
	// Path is the entity's hierarchy position within Parent, PathSeparator-joined
	// (x-clicky.surfaces[].path). Build it with JoinPath/SplitPath so the
	// separator stays the wire convention. Empty declares no hierarchy.
	Path string
	List func(opts ListOpts) ([]T, error)
	Get  func(id string) (R, error)
	// ListWithContext / GetWithContext / GetWithFlagsAndContext are context-aware
	// variants of List / Get / GetWithFlags. When set they take precedence over
	// their non-context counterparts and receive the request-scoped
	// context.Context (r.Context() over HTTP, cmd.Context() on the CLI). Use them
	// to resolve per-request state (e.g. a database/config bundle) instead of
	// reaching for process globals.
	ListWithContext        func(ctx context.Context, opts ListOpts) ([]T, error)
	ListPaged              func(opts ListOpts) (PagedResult[T], error)
	ListPagedWithContext   func(ctx context.Context, opts ListOpts) (PagedResult[T], error)
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
	// DeleteFlags, when non-nil, is a zero-value struct value implementing
	// ActionFlags whose `flag:"..."` tagged fields are registered as
	// CLI flags on the generated `delete` subcommand and published as query
	// parameters on its DELETE route. The parsed values are passed into
	// DeleteWithFlagsAndContext.
	//
	// DeleteWithFlagsAndContext takes precedence over DeleteWithContext and
	// Delete. Unlike them it returns a result, which the CLI renders and the
	// HTTP route answers with.
	DeleteFlags               ActionFlags
	DeleteWithFlagsAndContext func(ctx context.Context, id string, flags map[string]string) (any, error)
	Filters                   []Filter[ListOpts]
	// Sort enables validated server-side sorting on the list operation. Public
	// keys are derived from the response column metadata, never from SQL names.
	Sort *SortSpec

	PrimaryAction EntityAction
	Actions       []EntityAction
	BulkActions   []EntityBulkAction

	// Admin groups admin-only operations (inspect, configure, etc.)
	// that produce different views/columns from the main CRUD operations.
	// Generates subcommands under an "admin" subgroup and routes like /entity/admin/{verb}.
	Admin *Entity[T, ListOpts, R]

	// ValidArgs provides shell completion for the ID argument.
	ValidArgs func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

	// ToolGroup names the group all of this entity's operations belong to. It is
	// inherited by every generated operation; an action may override it via
	// WithToolGroup. AI tool-preference layers use it to toggle related tools
	// together.
	ToolGroup string
	ToolHints MCPToolHints

	// ServedOnly keeps this entity off the CLI while leaving it on the HTTP and
	// MCP surfaces. Set it when the entity's state lives inside the running
	// server — sessions it holds, runs it has in flight — so a command in
	// another process would answer about nothing. See MarkServedOnly.
	ServedOnly bool

	// Routes publishes individual operations at declared URLs or methods,
	// keyed by verb ("list", "get", "create", "update", "delete"). Use it for an
	// entity whose callers already address it at paths this registration did not
	// choose; actions declare theirs with ActionSpec.WithPath instead.
	Routes map[string]RouteOverride
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

// entityListOperation builds the "list" operation for an entity, or reports
// false when it declares no list func. An Admin sub-entity is the same Entity
// type, so it registers through here too — keeping one implementation is what
// stops the admin surface from silently lagging behind the main one.
func entityListOperation[T EntityItem, ListOpts any, R any](e Entity[T, ListOpts, R]) (EntityOperation, bool) {
	if e.ListPagedWithContext == nil && e.ListPaged == nil && e.ListWithContext == nil && e.List == nil {
		return EntityOperation{}, false
	}
	sortSpec := prepareSortSpec[T, ListOpts](e.Sort)
	op := EntityOperation{
		Verb:              "list",
		LookupFunc:        buildLookupFunc[ListOpts](e.Filters),
		ContextLookupFunc: buildLookupFuncWithContext[ListOpts](e.Filters),
		BindCompletions:   buildFilterCompletionBinder[ListOpts](e.Filters),
		ResponseType:      reflect.TypeOf((*T)(nil)).Elem(),
		ResponseArray:     true,
		ResponseEntityID:  true,
		Sort:              sortSpec,
	}
	switch {
	case e.ListPagedWithContext != nil:
		op.ResponsePaged = true
		op.ContextDataFunc = func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
			opts, err := resolveSortableEntityOpts[ListOpts](flagMap, e.Filters, sortSpec)
			if err != nil {
				return nil, err
			}
			page, err := e.ListPagedWithContext(ctx, opts)
			if err != nil {
				return nil, err
			}
			return NewPagedResult(withEntityIDs(page.Data), page.Page.Limit, page.Page.Offset, page.Page.Total), nil
		}
	case e.ListPaged != nil:
		op.ResponsePaged = true
		op.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
			opts, err := resolveSortableEntityOpts[ListOpts](flagMap, e.Filters, sortSpec)
			if err != nil {
				return nil, err
			}
			page, err := e.ListPaged(opts)
			if err != nil {
				return nil, err
			}
			return NewPagedResult(withEntityIDs(page.Data), page.Page.Limit, page.Page.Offset, page.Page.Total), nil
		}
	case e.ListWithContext != nil:
		op.ContextDataFunc = func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
			opts, err := resolveSortableEntityOpts[ListOpts](flagMap, e.Filters, sortSpec)
			if err != nil {
				return nil, err
			}
			items, err := e.ListWithContext(ctx, opts)
			if err != nil {
				return nil, err
			}
			return withEntityIDs(items), nil
		}
	default:
		op.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
			opts, err := resolveSortableEntityOpts[ListOpts](flagMap, e.Filters, sortSpec)
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
	return op, true
}

// entityGetOperation builds the "get" operation for an entity, or reports false
// when it declares no get func. Shared with Admin sub-entities — see
// entityListOperation.
func entityGetOperation[T EntityItem, ListOpts any, R any](e Entity[T, ListOpts, R]) (EntityOperation, bool) {
	op := EntityOperation{Verb: "get", ResponseType: responseTypeOf[R]()}
	switch {
	case e.GetWithFlagsAndContext != nil:
		op.FlagsType = actionFlagsType(e.GetFlags)
		op.ContextDataFunc = func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
			id, err := entityIDFrom(flagMap, args)
			if err != nil {
				return nil, err
			}
			return dataOrError(e.GetWithFlagsAndContext(ctx, id, flagMap))
		}
	case e.GetWithFlags != nil:
		op.FlagsType = actionFlagsType(e.GetFlags)
		op.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
			id, err := entityIDFrom(flagMap, args)
			if err != nil {
				return nil, err
			}
			return dataOrError(e.GetWithFlags(id, flagMap))
		}
	case e.GetWithContext != nil:
		op.ContextDataFunc = func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
			id, err := entityIDFrom(flagMap, args)
			if err != nil {
				return nil, err
			}
			return dataOrError(e.GetWithContext(ctx, id))
		}
	case e.Get != nil:
		op.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
			id, err := entityIDFrom(flagMap, args)
			if err != nil {
				return nil, err
			}
			return dataOrError(e.Get(id))
		}
	default:
		return EntityOperation{}, false
	}
	return op, true
}

// RegisterEntity registers a CRUD entity. Call during init().
// CLI commands and HTTP routes are generated later via GenerateCLI/GenerateRoutes.
//
// See the github.com/flanksource/clicky/entity package for the authoring guide
// and the best practices for operations that work well across CLI and HTTP.
func RegisterEntity[T EntityItem, ListOpts any, R any](e Entity[T, ListOpts, R]) {
	name := e.Name
	if name == "" {
		name = lo.KebabCase(reflect.TypeOf((*T)(nil)).Elem().Name())
	}

	info := EntityInfo{
		Name:       name,
		Parent:     e.Parent,
		Aliases:    e.Aliases,
		Path:       e.Path,
		Type:       reflect.TypeOf((*T)(nil)).Elem(),
		ListType:   reflect.TypeOf((*ListOpts)(nil)).Elem(),
		ValidArgs:  entityIDCompletion(e),
		IDOptions:  entityIDOptions(e),
		ServedOnly: e.ServedOnly,
		FilterRefs: entityFilterRefs(e.Filters),
		ToolGroup:  e.ToolGroup,
		ToolHints:  e.ToolHints,
	}
	if info.ToolHints.Group == "" {
		info.ToolHints.Group = info.ToolGroup
	}
	if info.ToolGroup == "" {
		info.ToolGroup = info.ToolHints.Group
	}

	if op, ok := entityListOperation(e); ok {
		info.Operations = append(info.Operations, op)
	}
	if op, ok := entityGetOperation(e); ok {
		info.Operations = append(info.Operations, op)
	}

	for _, build := range []func(Entity[T, ListOpts, R]) (EntityOperation, bool){
		entityCreateOperation[T, ListOpts, R],
		entityUpdateOperation[T, ListOpts, R],
		entityDeleteOperation[T, ListOpts, R],
	} {
		if op, ok := build(e); ok {
			info.Operations = append(info.Operations, op)
		}
	}

	if e.PrimaryAction != nil {
		primary := e.PrimaryAction.actionInfo()
		info.PrimaryAction = &primary
	}
	info.Actions = entityActionInfos(name, e.Actions)
	info.BulkActions = entityBulkActionInfos(e)

	if e.Admin != nil {
		registerAdminEntity(e, info)
	}

	// The parent's own operations; the admin sub-entity applied its own above.
	applyRouteOverrides(info.Operations, e.Routes)
	storeEntityInfo(info)
}

func storeEntityInfo(info EntityInfo) {
	observeEntity(&info)
	entityRegistryMu.Lock()
	entityRegistry = append(entityRegistry, info)
	entityRegistryMu.Unlock()
}

func entityCreateOperation[T EntityItem, ListOpts any, R any](e Entity[T, ListOpts, R]) (EntityOperation, bool) {
	op := EntityOperation{Verb: "create", ResponseType: responseTypeOf[R]()}
	switch {
	case e.CreateWithContext != nil:
		op.ContextDataFunc = func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
			return dataOrError(e.CreateWithContext(ctx, entityBody(flagMap)))
		}
	case e.Create != nil:
		op.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
			return dataOrError(e.Create(entityBody(flagMap)))
		}
	default:
		return EntityOperation{}, false
	}
	return op, true
}

func entityUpdateOperation[T EntityItem, ListOpts any, R any](e Entity[T, ListOpts, R]) (EntityOperation, bool) {
	op := EntityOperation{Verb: "update", ResponseType: responseTypeOf[R]()}
	switch {
	case e.UpdateWithContext != nil:
		op.ContextDataFunc = func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
			id, err := entityIDFrom(flagMap, args)
			if err != nil {
				return nil, err
			}
			return dataOrError(e.UpdateWithContext(ctx, id, entityBodyExcludingID(flagMap)))
		}
	case e.Update != nil:
		op.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
			id, err := entityIDFrom(flagMap, args)
			if err != nil {
				return nil, err
			}
			return dataOrError(e.Update(id, entityBodyExcludingID(flagMap)))
		}
	default:
		return EntityOperation{}, false
	}
	return op, true
}

func entityDeleteOperation[T EntityItem, ListOpts any, R any](e Entity[T, ListOpts, R]) (EntityOperation, bool) {
	op := EntityOperation{Verb: "delete"}
	switch {
	case e.DeleteWithFlagsAndContext != nil:
		op.FlagsType = actionFlagsType(e.DeleteFlags)
		op.ResponseType = responseTypeOf[any]()
		op.ContextDataFunc = func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
			id, err := entityIDFrom(flagMap, args)
			if err != nil {
				return nil, err
			}
			return dataOrError(e.DeleteWithFlagsAndContext(ctx, id, flagMap))
		}
	case e.DeleteWithContext != nil:
		op.ContextDataFunc = func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
			id, err := entityIDFrom(flagMap, args)
			if err != nil {
				return nil, err
			}
			return nil, e.DeleteWithContext(ctx, id)
		}
	case e.Delete != nil:
		op.DataFunc = func(flagMap map[string]string, args []string) (any, error) {
			id, err := entityIDFrom(flagMap, args)
			if err != nil {
				return nil, err
			}
			return nil, e.Delete(id)
		}
	default:
		return EntityOperation{}, false
	}
	return op, true
}

// entityActionInfos resolves an entity's actions. An action's verb becomes a
// subcommand name, so two actions sharing one are a wiring bug: cobra keeps
// whichever was added first and the other silently never runs, however
// different their routes are.
func entityActionInfos(name string, actions []EntityAction) []ActionInfo {
	var infos []ActionInfo
	verbs := make(map[string]bool, len(actions))
	for _, action := range actions {
		if action == nil {
			continue
		}
		actionInfo := action.actionInfo()
		if verbs[actionInfo.Name] {
			panic("clicky.RegisterEntity: entity " + name + " declares two actions with the verb " + actionInfo.Name)
		}
		verbs[actionInfo.Name] = true
		infos = append(infos, actionInfo)
	}
	return infos
}

func entityBulkActionInfos[T EntityItem, ListOpts any, R any](e Entity[T, ListOpts, R]) []BulkActionInfo {
	var infos []BulkActionInfo
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
		infos = append(infos, bai)
	}
	return infos
}

// registerAdminEntity stores e.Admin as a sub-entity of parent; GenerateCLI
// nests it under an "admin" command.
func registerAdminEntity[T EntityItem, ListOpts any, R any](e Entity[T, ListOpts, R], parent EntityInfo) {
	admin := *e.Admin
	if admin.Name == "" || admin.Name == parent.Name {
		admin.Name = parent.Name
	}
	adminValidArgs := admin.ValidArgs
	if adminValidArgs == nil {
		adminValidArgs = e.ValidArgs
	}
	adminInfo := EntityInfo{
		Name:      admin.Name,
		Type:      parent.Type,
		ListType:  parent.ListType,
		ValidArgs: adminValidArgs,
		IsAdmin:   true,
	}
	if op, ok := entityListOperation(admin); ok {
		adminInfo.Operations = append(adminInfo.Operations, op)
	}
	if op, ok := entityGetOperation(admin); ok {
		adminInfo.Operations = append(adminInfo.Operations, op)
	}
	for _, action := range admin.Actions {
		if action == nil {
			continue
		}
		adminInfo.Actions = append(adminInfo.Actions, action.actionInfo())
	}
	// The admin sub-entity is registered separately from its parent, so its
	// own overrides have to be applied here; the parent's call never sees these
	// operations.
	applyRouteOverrides(adminInfo.Operations, admin.Routes)
	storeEntityInfo(adminInfo)
}

// GetEntities returns all registered entities.
func GetEntities() []EntityInfo {
	entityRegistryMu.Lock()
	defer entityRegistryMu.Unlock()
	return append([]EntityInfo{}, entityRegistry...)
}

// GetEntity returns the registered entity with the given name (or alias) and
// whether it was found.
func GetEntity(name string) (EntityInfo, bool) {
	entityRegistryMu.Lock()
	defer entityRegistryMu.Unlock()
	for _, info := range entityRegistry {
		if info.Name == name {
			return info, true
		}
		for _, alias := range info.Aliases {
			if alias == name {
				return info, true
			}
		}
	}
	return EntityInfo{}, false
}

// filterRefProvider is implemented by filter adapters that reference a reusable
// named filter (the entity package's Use/As helper). It lets RegisterEntity
// record the param-key→filter-name mapping without importing that package.
type filterRefProvider interface {
	FilterName() string
}

// entityFilterRefs records, for each filter that references a reusable named
// filter, the mapping from its flag key to the named filter's name.
func entityFilterRefs[ListOpts any](filters []Filter[ListOpts]) map[string]string {
	refs := make(map[string]string)
	for _, f := range filters {
		if p, ok := f.(filterRefProvider); ok {
			if name := p.FilterName(); name != "" {
				refs[f.Key()] = name
			}
		}
	}
	if len(refs) == 0 {
		return nil
	}
	return refs
}
