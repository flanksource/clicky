package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/schemaform"
)

// OperationSchemaOptions configures OperationInputSchema.
type OperationSchemaOptions struct {
	// LookupFilters are the filter keys the operation's lookup serves
	// (OperationLookupFilters); those filter parameters get an x-clicky-lookup.
	LookupFilters []string
	// Completions are parameters with shell-completion choices; they get an
	// x-clicky-lookup without a URL — the terminal resolves it through the
	// command, and the web (which has no completion endpoint) renders a plain field.
	Completions []string
	// Positional are the parameters the command takes as positional args; they
	// get the "positional" x-clicky role, which the terminal form always shows.
	Positional []string
	// FlagSchemas maps JSON-object parameters to the scope their schema depends
	// on; they get an x-clicky-schema the terminal form loads (entity.FlagSchema).
	FlagSchemas map[string][]string
}

// OperationInputSchema is the JSON Schema of an operation's arguments: one
// property per parameter, in parameter order (x-order), each converted exactly
// as the OpenAPI document converts it — with its x-clicky role and
// x-clicky-lookup — plus any request-body properties the parameters do not
// cover. Its values feed CommandExecutor.ExecuteOperation unchanged.
func OperationInputSchema(op RPCOperation, opts OperationSchemaOptions) (json.RawMessage, error) {
	g := NewOpenAPIGenerator(nil)
	properties := map[string]any{}
	order := []string{}
	required := []string{}
	for _, param := range op.Parameters {
		converted := g.convertParameter(op, param)
		prop, err := schemaMap(converted.Schema)
		if err != nil {
			return nil, fmt.Errorf("%s: parameter %s: %w", op.Name, param.Name, err)
		}
		if containsString(opts.Positional, param.Name) {
			converted.Clicky = &ClickyParameterMeta{Role: "positional"}
		}
		if converted.Clicky != nil {
			prop["x-clicky"] = converted.Clicky
		}
		if scope, ok := opts.FlagSchemas[param.Name]; ok {
			prop["x-clicky-schema"] = schemaform.SchemaSource{Source: param.Name, Scope: scope}
		}
		if lookup := operationLookupMeta(op, param.Name, converted, opts.LookupFilters); lookup != nil {
			prop["x-clicky-lookup"] = lookup
		} else if containsString(opts.Completions, param.Name) {
			prop["x-clicky-lookup"] = &ClickyLookupMeta{Filter: param.Name}
		}
		properties[param.Name] = prop
		order = append(order, param.Name)
		if param.Required {
			required = append(required, param.Name)
		}
	}
	for _, name := range sortedPropertyNames(op.Schema.Properties) {
		if _, ok := properties[name]; ok {
			continue
		}
		prop, err := schemaMap(g.convertPropertyToOpenAPI(op.Schema.Properties[name]))
		if err != nil {
			return nil, fmt.Errorf("%s: property %s: %w", op.Name, name, err)
		}
		properties[name] = prop
		order = append(order, name)
	}
	for _, name := range op.Schema.Required {
		if !containsString(required, name) {
			required = append(required, name)
		}
	}
	return json.Marshal(map[string]any{
		"type":       "object",
		"title":      op.Name,
		"properties": properties,
		"required":   required,
		"x-order":    order,
	})
}

// operationLookupMeta is the lookup the terminal form resolves for a filter
// parameter: the named-filter meta the OpenAPI document carries, or — for a
// convention filter the operation's lookup actually serves — the same
// `__lookup_filter` request the web FilterBar makes. Every list query param
// has the filter role once an op supports lookups, so the served keys decide.
func operationLookupMeta(op RPCOperation, name string, converted OpenAPIParameter, served []string) *ClickyLookupMeta {
	if converted.Lookup != nil {
		return converted.Lookup
	}
	if converted.Clicky == nil || converted.Clicky.Role != "filter" || !containsString(served, name) {
		return nil
	}
	return &ClickyLookupMeta{URL: op.Path, Filter: name, SearchParam: "__lookup_q"}
}

// OperationLookupFilters asks the operation's lookup which filters it serves,
// as the web FilterBar's `?__lookup=filters` request does. Nil when the
// operation serves no lookups.
func OperationLookupFilters(ctx context.Context, op RPCOperation) ([]string, error) {
	var data any
	var err error
	switch {
	case op.ContextLookupFunc != nil:
		data, err = op.ContextLookupFunc(ctx, map[string]string{}, nil)
	case op.LookupFunc != nil:
		data, err = op.LookupFunc(map[string]string{}, nil)
	default:
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: lookup filters: %w", op.Name, err)
	}
	response, err := asLookupResponse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op.Name, err)
	}
	keys := make([]string, 0, len(response.Filters))
	for key := range response.Filters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}

func asLookupResponse(data any) (entity.LookupResponse, error) {
	switch typed := data.(type) {
	case entity.LookupResponse:
		return typed, nil
	case *entity.LookupResponse:
		return *typed, nil
	}
	return entity.LookupResponse{}, fmt.Errorf("lookup returned %T, expected entity.LookupResponse", data)
}

func schemaMap(schema *OpenAPISchema) (map[string]any, error) {
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	return out, json.Unmarshal(raw, &out)
}

func sortedPropertyNames(properties map[string]Property) []string {
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func containsString(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// OperationLookup resolves the form's x-clicky-lookup fields through the
// operation's own lookup func, the way the `?__lookup=filters` endpoint does:
// sibling values scope the lookup, `__lookup_filter`/`__lookup_q` narrow it to
// one filter and a search term. Nil when the operation serves no lookups.
func OperationLookup(op RPCOperation) schemaform.Lookup {
	if op.LookupFunc == nil && op.ContextLookupFunc == nil {
		return nil
	}
	return func(ctx context.Context, req schemaform.LookupRequest) (schemaform.LookupResult, error) {
		flags := map[string]string{"__lookup_filter": req.Spec.Filter}
		if req.Query != "" {
			flags["__lookup_q"] = req.Query
		}
		var args []string
		for name, value := range req.Values {
			switch name {
			case "args":
				args = operationArgs(value)
			case req.Spec.Filter:
			default:
				flags[name] = convertValueToString(value)
			}
		}
		var data any
		var err error
		if op.ContextLookupFunc != nil {
			data, err = op.ContextLookupFunc(ctx, flags, args)
		} else {
			data, err = op.LookupFunc(flags, args)
		}
		if err != nil {
			return schemaform.LookupResult{}, err
		}
		return lookupResult(data, req.Spec.Filter)
	}
}

func lookupResult(data any, filter string) (schemaform.LookupResult, error) {
	response, err := asLookupResponse(data)
	if err != nil {
		return schemaform.LookupResult{}, err
	}
	f, ok := response.Filters[filter]
	if !ok {
		return schemaform.LookupResult{}, fmt.Errorf("lookup returned no filter %q", filter)
	}
	values := make([]string, 0, len(f.Options))
	for value := range f.Options {
		values = append(values, value)
	}
	sort.Strings(values)
	options := make([]schemaform.Option, len(values))
	for i, value := range values {
		label := f.Options[value].Plain
		if label == "" {
			label = f.Options[value].Text
		}
		if label == "" || label == value {
			label = value
		} else {
			label = label + " (" + value + ")"
		}
		options[i] = schemaform.Option{Value: value, Label: label, Raw: value}
	}
	return schemaform.LookupResult{Options: options, Multi: f.Multi, Truncated: f.Truncated, Total: f.Total}, nil
}

// PromptOperationOptions configures PromptOperation.
type PromptOperationOptions struct {
	// Preset pre-fills the form (e.g. the flags given on the command line).
	Preset map[string]any
	// Completions offer choices for arguments the operation's lookup does not
	// serve (commandCompletions).
	Completions map[string]Completion
	// Positional names the parameters taken as positional args.
	Positional []string
	// FlagSchemas shape JSON-object flags from the other arguments; their fields
	// are loaded into the form as those arguments are answered.
	FlagSchemas map[string]entity.FlagSchema
}

// PromptOperation collects an operation's arguments interactively — the huh
// form on a terminal, the JsonSchemaForm prompt on a dashboard sink. The values
// are ready for ExecuteOperation.
func PromptOperation(ctx context.Context, op RPCOperation, opts PromptOperationOptions) (map[string]any, error) {
	served, err := OperationLookupFilters(ctx, op)
	if err != nil {
		return nil, err
	}
	completions := make([]string, 0, len(opts.Completions))
	for name := range opts.Completions {
		completions = append(completions, name)
	}
	schema, err := OperationInputSchema(op, OperationSchemaOptions{LookupFilters: served, Completions: completions, Positional: opts.Positional, FlagSchemas: flagSchemaScopes(opts.FlagSchemas)})
	if err != nil {
		return nil, err
	}
	return clicky.PromptForm(ctx, clicky.PromptFormOptions{
		Title:       op.Name,
		Description: op.Description,
		Schema:      schema,
		Default:     opts.Preset,
		Lookup:      formLookup(op, served, opts.Completions),
		LoadSchema:  flagSchemaLoader(opts.FlagSchemas),
	})
}

func flagSchemaScopes(schemas map[string]entity.FlagSchema) map[string][]string {
	out := make(map[string][]string, len(schemas))
	for name, schema := range schemas {
		out[name] = schema.Scope
	}
	return out
}

// flagSchemaLoader answers the form's x-clicky-schema requests from the
// command's flag schemas, with the scope values as the command would see them.
func flagSchemaLoader(schemas map[string]entity.FlagSchema) schemaform.SchemaLoader {
	if len(schemas) == 0 {
		return nil
	}
	return func(ctx context.Context, req schemaform.SchemaRequest) (json.RawMessage, error) {
		schema, ok := schemas[req.Source]
		if !ok {
			return nil, fmt.Errorf("no flag schema for --%s", req.Source)
		}
		values := make(map[string]string, len(req.Values))
		for name, v := range req.Values {
			values[name] = convertValueToString(v)
		}
		return schema.Load(ctx, values)
	}
}

// formLookup resolves a field's lookup through the operation's lookup when it
// serves that filter (richer labels, server-side search), else through the
// argument's shell completion.
func formLookup(op RPCOperation, served []string, completions map[string]Completion) schemaform.Lookup {
	operation := OperationLookup(op)
	return func(ctx context.Context, req schemaform.LookupRequest) (schemaform.LookupResult, error) {
		if operation != nil && containsString(served, req.Spec.Filter) {
			return operation(ctx, req)
		}
		complete, ok := completions[req.Spec.Filter]
		if !ok {
			return schemaform.LookupResult{}, fmt.Errorf("no lookup or completion serves %q", req.Spec.Filter)
		}
		options, err := complete(ctx, req.Values)
		if err != nil {
			return schemaform.LookupResult{}, err
		}
		return schemaform.LookupResult{Options: options}, nil
	}
}
