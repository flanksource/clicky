package entity

import (
	"context"
	"reflect"

	"github.com/flanksource/clicky/api"
)

// DynamicFilter describes one filter on a dynamic (schema-driven) entity. The
// Options/Selected closures resolve from the request flag map — they are built by
// the entity package from a reusable named filter — so the type-agnostic filter
// logic stays out of this package while reusing the shared lookup builder.
type DynamicFilter struct {
	Key        string
	Label      string
	Type       string
	Multi      bool
	Searchable bool
	// Limit caps the option set this filter enumerates in one shot. Zero takes
	// MaxLookupOptions, which is also the ceiling.
	Limit int
	// Options returns the head set (query == "") or search matches, plus the true
	// total behind the head. Required.
	Options func(ctx context.Context, flags map[string]string, query string, limit int) (map[string]api.Textable, int, error)
	// Selected labels the currently-selected value(s) of this filter's key.
	// Optional; nil means "no selection rendering".
	Selected func(ctx context.Context, flags map[string]string) (map[string]api.Textable, error)
}

func describeDynamicFilter(f DynamicFilter) (string, bool) { return f.Key, f.Searchable }

// DynamicEntitySpec is the type-erased description of a schema-driven entity,
// assembled by the entity package's NewDynamicEntity builder. It reuses the
// existing pipeline: ListType drives CLI flag binding and OpenAPI parameters,
// ItemType drives the response schema, and Filters feed the shared lookup engine.
type DynamicEntitySpec struct {
	Name    string
	Parent  string
	Aliases []string
	// Icon is an opaque UI icon name emitted on the surface (x-clicky.surfaces[].icon).
	Icon string
	// Path is the hierarchy position emitted on the surface (x-clicky.surfaces[].path).
	Path string
	// Title overrides the auto-generated surface title when non-empty.
	Title    string
	ListType reflect.Type // generated struct type: flag binding + OpenAPI params
	ItemType reflect.Type // generated item struct type for the response schema
	List     ContextDataFunc
	Get      ContextDataFunc
	Filters  []DynamicFilter
	// FilterRefs maps a filter key to the reusable named filter backing it, so
	// the OpenAPI layer can emit an x-clicky-lookup $ref (see EntityInfo.FilterRefs).
	FilterRefs map[string]string
}

// RegisterDynamicEntity registers a schema-driven entity assembled at runtime
// (no compile-time Go struct). It is the single root entrypoint the entity
// package's dynamic builder targets; everything downstream — GenerateCLI, RPC,
// OpenAPI, the lookup endpoint — consumes it through the normal entity registry.
func RegisterDynamicEntity(spec DynamicEntitySpec) {
	if spec.Name == "" {
		panic("clicky.RegisterDynamicEntity: spec.Name must not be empty")
	}
	if spec.ListType == nil || spec.ListType.Kind() != reflect.Struct {
		panic("clicky.RegisterDynamicEntity: spec.ListType must be a struct type")
	}
	if spec.List == nil {
		panic("clicky.RegisterDynamicEntity: spec.List must not be nil")
	}
	for _, f := range spec.Filters {
		if f.Options == nil {
			panic("clicky.RegisterDynamicEntity: DynamicFilter " + f.Key + " has no Options func")
		}
	}

	responseType := spec.ItemType
	if responseType == nil {
		responseType = spec.ListType
	}

	info := EntityInfo{
		Name:       spec.Name,
		Parent:     spec.Parent,
		Aliases:    spec.Aliases,
		Icon:       spec.Icon,
		Path:       spec.Path,
		Title:      spec.Title,
		Type:       responseType,
		ListType:   spec.ListType,
		FilterRefs: spec.FilterRefs,
	}

	listOp := EntityOperation{
		Verb:             "list",
		ContextDataFunc:  spec.List,
		ResponseType:     responseType,
		ResponseArray:    true,
		ResponseEntityID: true,
	}
	if len(spec.Filters) > 0 {
		listOp.LookupFunc = buildDynamicLookup(spec.Filters)
		listOp.ContextLookupFunc = buildDynamicContextLookup(spec.Filters)
	}
	info.Operations = append(info.Operations, listOp)

	if spec.Get != nil {
		info.Operations = append(info.Operations, EntityOperation{
			Verb:            "get",
			ContextDataFunc: spec.Get,
			ResponseType:    responseType,
		})
	}

	entityRegistryMu.Lock()
	entityRegistry = append(entityRegistry, info)
	entityRegistryMu.Unlock()
}

func buildDynamicContextLookup(filters []DynamicFilter) ContextLookupFunc {
	return func(ctx context.Context, flagMap map[string]string, _ []string) (any, error) {
		return resolveDynamicLookup(ctx, filters, flagMap)
	}
}

func buildDynamicLookup(filters []DynamicFilter) func(map[string]string, []string) (any, error) {
	return func(flagMap map[string]string, _ []string) (any, error) {
		return resolveDynamicLookup(context.Background(), filters, flagMap)
	}
}

func resolveDynamicLookup(ctx context.Context, filters []DynamicFilter, flagMap map[string]string) (entityLookupResponse, error) {
	searchKey := flagMap[lookupFilterKeyParam]
	searchQuery := flagMap[lookupQueryParam]
	delete(flagMap, lookupFilterKeyParam)
	delete(flagMap, lookupQueryParam)

	// Narrowed here as well as in resolveLookupCore, because Selected is resolved
	// eagerly: a dynamic filter's selection labels can be a backend round trip of
	// their own, and a targeted search discards every entry but one.
	if target := searchTarget(filters, describeDynamicFilter, searchKey, searchQuery); target >= 0 {
		filters = filters[target : target+1]
	}

	bound := make([]boundFilter, 0, len(filters))
	for _, df := range filters {
		df := df
		var selected map[string]api.Textable
		if df.Selected != nil {
			var err error
			selected, err = df.Selected(ctx, flagMap)
			if err != nil {
				return entityLookupResponse{}, err
			}
		}
		bound = append(bound, boundFilter{
			Key:        df.Key,
			Label:      df.Label,
			Type:       df.Type,
			Multi:      df.Multi,
			Searchable: df.Searchable,
			Limit:      df.Limit,
			Selected:   selected,
			Options: func(query string, limit int) (map[string]api.Textable, int, error) {
				return df.Options(ctx, flagMap, query, limit)
			},
		})
	}
	return resolveLookupCore(bound, searchKey, searchQuery)
}
