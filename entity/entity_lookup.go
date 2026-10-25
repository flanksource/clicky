package entity

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/flags"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func buildFilterCompletionBinder[T any](filters []Filter[T]) func(cmd *cobra.Command) {
	return buildFilterCompletionBinderWithOptions(filters, nil)
}

func buildFilterCompletionBinderWithOptions[T any](filters []Filter[T], prepare func(context.Context, []string, *T)) func(cmd *cobra.Command) {
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

				opts, err := buildOpts[T](flagMap)
				if err != nil {
					return nil, cobra.ShellCompDirectiveError
				}
				if prepare != nil {
					prepare(cmd.Context(), args, &opts)
				}
				if _, err := applyEntityFilters(&opts, filters); err != nil {
					return nil, cobra.ShellCompDirectiveError
				}

				ctx := cmd.Context()
				if ctx == nil {
					ctx = context.Background()
				}
				answer, err := typedFilterOptions(ctx, filter, filterOptionsRequest[T]{
					Opts:       opts,
					Searchable: filterIsSearchable(filter),
					Query:      toComplete,
					Limit:      lookupOptionsLimit,
				})
				if err != nil {
					cobra.CompErrorln(err.Error())
					return nil, cobra.ShellCompDirectiveError
				}
				options := answer.Options
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

// BuildOpts creates an instance of T and populates it from a flag map, using
// the same `flag:`/`default:` tags the CLI binding reads.
//
// It is exported as the companion to WithFlags: an action that declares typed
// parameters is handed the raw map, and decoding it by hand would mean
// repeating every flag name the struct already declares — which is exactly the
// drift the typed flags exist to prevent.
func BuildOpts[T any](flagMap map[string]string) (T, error) {
	return buildOpts[T](flagMap)
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
//
// It is a ceiling rather than a target: a filter that knows its own cardinality
// says so with its own smaller Limit, and this is what remains for the ones that
// do not.
const lookupOptionsLimit = 200

// MaxLookupOptions is lookupOptionsLimit exported, so a caller declaring a
// per-filter Limit can validate it against the ceiling rather than discovering
// at request time that it was silently reduced.
const MaxLookupOptions = lookupOptionsLimit

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
	spec := newLookupSpec(filters)
	return func(flagMap map[string]string, args []string) (any, error) {
		return resolveLookup(context.Background(), spec, flagMap)
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
	spec := newLookupSpec(filters)
	return func(ctx context.Context, flagMap map[string]string, args []string) (any, error) {
		return resolveLookup(ctx, spec, flagMap)
	}
}

// resolveLookup builds the entity lookup response for one request. It strips
// the reserved search params, resolves opts + selected values, then fills each
// filter's option set — preferring the context-aware Options methods when the
// filter implements them, otherwise the plain Options/OptionsWithQuery.
func resolveLookup[T any](ctx context.Context, spec lookupSpec[T], flagMap map[string]string) (any, error) {
	search := takeLookupSearch(flagMap)
	opts, err := buildOpts[T](flagMap)
	if err != nil {
		return nil, err
	}
	return resolveLookupOptions(ctx, spec, opts, search)
}

// lookupSpec is the per-entity lookup configuration: the filters and the
// flag-tag metadata derived once from their ListOpts type.
type lookupSpec[T any] struct {
	Filters  []Filter[T]
	Metadata map[string]entityLookupMetadata
}

func newLookupSpec[T any](filters []Filter[T]) lookupSpec[T] {
	return lookupSpec[T]{Filters: filters, Metadata: buildLookupMetadata[T]()}
}

// lookupSearch is the single filter a lookup request searches server-side.
type lookupSearch struct {
	Key   string
	Query string
}

// takeLookupSearch reads the reserved search params and strips them from the
// flag map: they aren't entity flags, so they must never leak into buildOpts.
func takeLookupSearch(flagMap map[string]string) lookupSearch {
	search := lookupSearch{Key: flagMap[lookupFilterKeyParam], Query: flagMap[lookupQueryParam]}
	delete(flagMap, lookupFilterKeyParam)
	delete(flagMap, lookupQueryParam)
	return search
}

func resolveLookupOptions[T any](ctx context.Context, spec lookupSpec[T], opts T, search lookupSearch) (any, error) {
	filters := spec.Filters
	selected, err := applyEntityFilters(&opts, filters)
	if err != nil {
		return nil, err
	}

	bound := make([]boundFilter, 0, len(filters))
	for _, filter := range filters {
		meta := spec.Metadata[filter.Key()]
		if typed, ok := filter.(TypedFilter[T]); ok {
			if lookupType := typed.LookupType(); lookupType != "" {
				meta.Type = lookupType
			}
		}
		if unit, ok := filter.(UnitFilter); ok {
			meta.Unit = unit.LookupUnit()
		}
		if operator, ok := filter.(DefaultOperatorFilter); ok {
			meta.DefaultOperator = operator.LookupDefaultOperator()
		}
		if err := validateDefaultOperator(meta.Type, meta.DefaultOperator); err != nil {
			return nil, fmt.Errorf("filter %q: %w", filter.Key(), err)
		}
		f := filter
		searchable := filterIsSearchable(f)
		limit := 0
		if limited, ok := filter.(LimitedFilter); ok {
			limit = limited.LookupLimit()
		}
		bound = append(bound, boundFilter{
			Key:             f.Key(),
			Label:           f.Label(),
			Type:            meta.Type,
			Unit:            meta.Unit,
			DefaultOperator: meta.DefaultOperator,
			Multi:           meta.Multi,
			Searchable:      searchable,
			Limit:           limit,
			Selected:        selected[f.Key()],
			Options: func(query string, limit int) (FilterOptions, error) {
				return typedFilterOptions(ctx, f, filterOptionsRequest[T]{Opts: opts, Searchable: searchable, Query: query, Limit: limit})
			},
		})
	}

	return resolveLookupCore(bound, search.Key, search.Query)
}

// typedFilterOptions answers one typed filter's lookup. A named filter attached
// with Use answers from its source, so the source's counts and errors reach the
// response; every other typed filter only has a map of options to give.
type filterOptionsProvider[T any] interface {
	lookupOptions(ctx context.Context, opts T, query string, limit int) (FilterOptions, error)
}

// filterOptionsRequest is one filter option lookup: the resolved opts plus the
// search query and result limit for a searchable filter.
type filterOptionsRequest[T any] struct {
	Opts       T
	Searchable bool
	Query      string
	Limit      int
}

func typedFilterOptions[T any](ctx context.Context, filter Filter[T], req filterOptionsRequest[T]) (FilterOptions, error) {
	if provider, ok := filter.(filterOptionsProvider[T]); ok {
		return provider.lookupOptions(ctx, req.Opts, req.Query, req.Limit)
	}
	if fallible, ok := filter.(FallibleFilter[T]); ok {
		options, err := fallible.LookupOptions(ctx, req.Opts)
		if err != nil {
			return FilterOptions{}, fmt.Errorf("%s options: %w", filter.Key(), err)
		}
		return FilterOptions{Options: options, Total: len(options)}, nil
	}
	if req.Searchable {
		options, total := filterOptionsWithQuery(ctx, filter, req)
		return FilterOptions{Options: options, Total: total}, nil
	}
	options := filterOptions(ctx, filter, req.Opts)
	return FilterOptions{Options: options, Total: len(options)}, nil
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
func filterOptionsWithQuery[T any](ctx context.Context, filter Filter[T], req filterOptionsRequest[T]) (map[string]api.Textable, int) {
	if c, ok := filter.(ContextSearchableFilter[T]); ok {
		return c.OptionsWithQueryAndContext(ctx, req.Opts, req.Query, req.Limit)
	}
	if s, ok := filter.(SearchableFilter[T]); ok {
		return s.OptionsWithQuery(req.Opts, req.Query, req.Limit)
	}
	options := filterOptions(ctx, filter, req.Opts)
	return options, len(options)
}

type entityLookupMetadata struct {
	Multi           bool
	Type            string
	Unit            string
	DefaultOperator string
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
	for fieldType.Kind() == reflect.Pointer {
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
