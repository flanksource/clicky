package rpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/flags"
)

// EntityOperationDTO is the JSON shape of a registered CRUD operation.
type EntityOperationDTO struct {
	Verb string `json:"verb"`
}

// EntityActionDTO is the JSON shape of a registered action or bulk action.
//
// It carries enough for a front end to *render* the action, not just name it:
// the presentation hints the action declared, whether it accepts a filtered
// selection, and a JSON Schema for its parameters. Without those a UI ends up
// re-declaring every action it wants to show — which is the drift this endpoint
// exists to prevent.
type EntityActionDTO struct {
	Name  string `json:"name"`
	Short string `json:"short,omitempty"`
	// Method and Path are the route this action is actually reachable at.
	//
	// They are published because a caller cannot derive them: the HTTP method
	// is inferred from the action's name, so an action called "delete" is a
	// DELETE while its siblings are POSTs. A front end that guessed POST for
	// everything would 404 on exactly the destructive action it most needs to
	// get right. Empty when the server was built without an executor, since
	// then there is no route to name.
	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`
	// SupportsFilterMode reports that the action accepts a filtered selection
	// as well as explicit ids — what lets a caller say "every row matching
	// these filters" without enumerating them.
	SupportsFilterMode bool `json:"supports_filter_mode,omitempty"`
	// ToolHints are the action's declared Icon / Group / DestructiveHint /
	// DefaultPermission. Named for MCP, but they are exactly the metadata a
	// selection toolbar needs, so nothing second is invented for the UI.
	ToolHints *clicky.MCPToolHints `json:"tool_hints,omitempty"`
	// ParamSchema is a JSON Schema for the action's own parameters, or nil when
	// it declares none. Renderable directly by a JSON-schema form.
	ParamSchema *ActionParamSchema `json:"param_schema,omitempty"`
}

// ActionParamSchema is a JSON Schema object describing an action's parameters.
type ActionParamSchema struct {
	Type       string                      `json:"type"`
	Properties map[string]ActionParamField `json:"properties"`
	Required   []string                    `json:"required,omitempty"`
}

type ActionParamField struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Default     string   `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Items       *struct {
		Type string `json:"type"`
	} `json:"items,omitempty"`
}

// actionParamSchema reflects an action's flags struct into a JSON Schema.
// It reads the same `flag:`/`help:`/`default:`/`required:`/`enum:` tags the CLI
// binding reads, so the form a UI renders and the flags the CLI accepts cannot
// describe different parameters.
func actionParamSchema(t reflect.Type) *ActionParamSchema {
	if t == nil {
		return nil
	}
	fields, err := flags.ParseStructFields(t)
	if err != nil || len(fields) == 0 {
		return nil
	}
	schema := &ActionParamSchema{Type: "object", Properties: map[string]ActionParamField{}}
	for _, field := range fields {
		// Hidden fields are absent from --help and the OpenAPI schema; a form is
		// no different. `args`/`stdin` fields are not parameters at all.
		if field.FlagName == "" || field.Hidden || field.IsArgs || field.IsStdin {
			continue
		}
		prop := ActionParamField{
			Type:        jsonSchemaType(field.FieldType),
			Description: field.Help,
			Default:     field.DefaultValue,
			Enum:        field.Enum,
		}
		if prop.Type == "array" {
			prop.Items = &struct {
				Type string `json:"type"`
			}{Type: jsonSchemaType(field.FieldType.Elem())}
		}
		schema.Properties[field.FlagName] = prop
		if field.Required {
			schema.Required = append(schema.Required, field.FlagName)
		}
	}
	if len(schema.Properties) == 0 {
		return nil
	}
	return schema
}

func jsonSchemaType(t reflect.Type) string {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return "array"
	default:
		// Durations, enums-as-strings and anything else a flag can carry all
		// arrive as text on the wire.
		return "string"
	}
}

// toolHintsOrNil drops an all-zero hints struct so the DTO omits the key rather
// than shipping an object of empty fields.
func toolHintsOrNil(hints clicky.MCPToolHints) *clicky.MCPToolHints {
	if hints == (clicky.MCPToolHints{}) {
		return nil
	}
	return &hints
}

// EntityDTO is the JSON-serialisable projection of clicky.EntityInfo.
// reflect.Type and function fields are stripped so the registry can be
// exposed to HTTP clients (typically the web UI building its sidebar).
type EntityDTO struct {
	Name        string               `json:"name"`
	Aliases     []string             `json:"aliases,omitempty"`
	Parent      string               `json:"parent,omitempty"`
	IsAdmin     bool                 `json:"is_admin,omitempty"`
	Operations  []EntityOperationDTO `json:"operations,omitempty"`
	Actions     []EntityActionDTO    `json:"actions,omitempty"`
	BulkActions []EntityActionDTO    `json:"bulk_actions,omitempty"`
}

// EntitySnapshot returns a serialisable projection of the current entity
// registry. Callers that need to embed the registry in something other
// than the HTTP handler (tests, custom routes) can call this directly.
func EntitySnapshot() []EntityDTO {
	entities := clicky.GetEntities()
	out := make([]EntityDTO, 0, len(entities))
	for _, e := range entities {
		dto := EntityDTO{
			Name:    e.Name,
			Aliases: e.Aliases,
			Parent:  e.Parent,
			IsAdmin: e.IsAdmin,
		}
		for _, op := range e.Operations {
			dto.Operations = append(dto.Operations, EntityOperationDTO{Verb: op.Verb})
		}
		for _, a := range e.Actions {
			dto.Actions = append(dto.Actions, EntityActionDTO{
				Name:        a.Name,
				Short:       a.Short,
				ToolHints:   toolHintsOrNil(a.ToolHints),
				ParamSchema: actionParamSchema(a.FlagsType),
			})
		}
		for _, b := range e.BulkActions {
			dto.BulkActions = append(dto.BulkActions, EntityActionDTO{
				Name:               b.Name,
				Short:              b.Short,
				SupportsFilterMode: b.FilterFunc != nil || b.ContextFilterFunc != nil,
				ToolHints:          toolHintsOrNil(b.ToolHints),
				ParamSchema:        actionParamSchema(b.FlagsType),
			})
		}
		out = append(out, dto)
	}
	return out
}

// entitySnapshotWithRoutes annotates each action with the route it is really
// served at, read from the operations the executor registered rather than
// re-derived. Re-deriving would be a second implementation of the method
// inference, free to drift from the one that registered the handlers — and the
// symptom of that drift is a 404 on a route the catalog swore existed.
func (s *SwaggerServer) entitySnapshotWithRoutes() []EntityDTO {
	snapshot := EntitySnapshot()
	if s == nil || s.executor == nil || s.executor.service == nil {
		return snapshot
	}

	type route struct{ method, path string }
	routes := map[string]route{}
	for _, operation := range s.executor.service.Operations {
		meta := operation.Clicky
		if meta == nil || meta.Entity == "" || meta.ActionName == "" {
			continue
		}
		routes[meta.Entity+"/"+meta.ActionName] = route{operation.Method, operation.Path}
	}

	annotate := func(entity string, actions []EntityActionDTO) {
		for i := range actions {
			if found, ok := routes[entity+"/"+actions[i].Name]; ok {
				actions[i].Method, actions[i].Path = found.method, found.path
			}
		}
	}
	for i := range snapshot {
		annotate(snapshot[i].Name, snapshot[i].Actions)
		annotate(snapshot[i].Name, snapshot[i].BulkActions)
	}
	return snapshot
}

// handleEntities serves the entity registry snapshot as JSON. Mounted at
// /api/entities by SwaggerServer.RegisterRoutes so UIs can enumerate
// registered entities without parsing the OpenAPI spec.
func (s *SwaggerServer) handleEntities(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		return
	}

	body, err := json.MarshalIndent(s.entitySnapshotWithRoutes(), "", "  ")
	if err != nil {
		s.writeError(w, r, fmt.Errorf("encode entities: %w", err))
		return
	}
	_, _ = w.Write(body)
}
