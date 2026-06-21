package rpc

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/entity"
)

// OpenAPISpec represents an OpenAPI 3.0 specification
type OpenAPISpec struct {
	OpenAPI    string                 `json:"openapi"`
	Info       OpenAPIInfo            `json:"info"`
	Servers    []OpenAPIServer        `json:"servers,omitempty"`
	Paths      map[string]OpenAPIPath `json:"paths"`
	Components *OpenAPIComponents     `json:"components,omitempty"`
	Tags       []OpenAPITag           `json:"tags,omitempty"`
	Clicky     *ClickySpecMeta        `json:"x-clicky,omitempty"`
}

// OpenAPIInfo contains metadata about the API
type OpenAPIInfo struct {
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	Version     string          `json:"version"`
	Contact     *OpenAPIContact `json:"contact,omitempty"`
	License     *OpenAPILicense `json:"license,omitempty"`
}

// OpenAPIContact contains contact information for the API
type OpenAPIContact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// OpenAPILicense contains license information for the API
type OpenAPILicense struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// OpenAPIServer represents a server in the OpenAPI spec
type OpenAPIServer struct {
	URL         string                     `json:"url"`
	Description string                     `json:"description,omitempty"`
	Variables   map[string]OpenAPIVariable `json:"variables,omitempty"`
}

// OpenAPIVariable represents a server variable
type OpenAPIVariable struct {
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default"`
	Description string   `json:"description,omitempty"`
}

// OpenAPIPath represents a path item in the OpenAPI spec
type OpenAPIPath map[string]OpenAPIOperation

// OpenAPIOperation represents an operation in the OpenAPI spec
type OpenAPIOperation struct {
	Tags        []string                   `json:"tags,omitempty"`
	Summary     string                     `json:"summary,omitempty"`
	Description string                     `json:"description,omitempty"`
	OperationID string                     `json:"operationId,omitempty"`
	Parameters  []OpenAPIParameter         `json:"parameters,omitempty"`
	RequestBody *OpenAPIRequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]OpenAPIResponse `json:"responses"`
	Security    []map[string][]string      `json:"security,omitempty"`
	Clicky      *ClickyOperationMeta       `json:"x-clicky,omitempty"`
}

// OpenAPIParameter represents a parameter in the OpenAPI spec
type OpenAPIParameter struct {
	Name        string               `json:"name"`
	In          string               `json:"in"`
	Description string               `json:"description,omitempty"`
	Required    bool                 `json:"required,omitempty"`
	Schema      *OpenAPISchema       `json:"schema,omitempty"`
	Example     interface{}          `json:"example,omitempty"`
	Clicky      *ClickyParameterMeta `json:"x-clicky,omitempty"`
	Lookup      *ClickyLookupMeta    `json:"x-clicky-lookup,omitempty"`
}

// ClickyLookupMeta is the x-clicky-lookup extension on a filter parameter. It
// references the reusable named-filter definition via Ref and tells the client
// where to fetch options: URL is the owning entity's list path, which serves the
// lookup via the ?__lookup=filters convention with Filter/SearchParam as the
// per-filter search keys.
type ClickyLookupMeta struct {
	// Ref points at the reusable filter definition in
	// components.x-clicky-filters (e.g. "#/components/x-clicky-filters/users").
	Ref string `json:"$ref,omitempty"`
	// URL is the lookup endpoint (the owning entity's list path).
	URL string `json:"url,omitempty"`
	// Filter is the bound flag key, sent as __lookup_filter for server-side search.
	Filter string `json:"filter,omitempty"`
	// SearchParam is the query-string key for the search term (__lookup_q).
	SearchParam string `json:"searchParam,omitempty"`
	// Multi reports whether the control accepts multiple selections.
	Multi bool `json:"multi,omitempty"`
}

// ClickyParameterMeta carries UI hints for a single OpenAPI parameter so the
// explorer can route it to the right widget instead of treating every query
// param as a free-text input. Role is derived from the parameter name plus
// the operation's lookup capability — see paramRole.
type ClickyParameterMeta struct {
	// Role tells the UI how to render this parameter. Empty means "no
	// special handling". Recognised values:
	//   - "filter"     — show as a filter pill in the DataTable's FilterBar
	//                    (entity .Filters(...) keys or any list-op query param
	//                    when the op declares SupportsLookup).
	//   - "limit"      — drives DataTable pagination's pageSize.
	//   - "offset"     — drives DataTable pagination's page.
	//   - "time-from"  — left edge of a time-range picker.
	//   - "time-to"    — right edge of a time-range picker.
	Role string `json:"role,omitempty"`
}

// OpenAPIRequestBody represents a request body in the OpenAPI spec
type OpenAPIRequestBody struct {
	Description string                      `json:"description,omitempty"`
	Content     map[string]OpenAPIMediaType `json:"content"`
	Required    bool                        `json:"required,omitempty"`
}

// OpenAPIResponse represents a response in the OpenAPI spec
type OpenAPIResponse struct {
	Description string                      `json:"description"`
	Headers     map[string]OpenAPIHeader    `json:"headers,omitempty"`
	Content     map[string]OpenAPIMediaType `json:"content,omitempty"`
}

// OpenAPIHeader represents a header in the OpenAPI spec
type OpenAPIHeader struct {
	Description string         `json:"description,omitempty"`
	Required    bool           `json:"required,omitempty"`
	Schema      *OpenAPISchema `json:"schema,omitempty"`
}

// OpenAPIMediaType represents a media type in the OpenAPI spec
type OpenAPIMediaType struct {
	Schema   *OpenAPISchema            `json:"schema,omitempty"`
	Example  interface{}               `json:"example,omitempty"`
	Examples map[string]OpenAPIExample `json:"examples,omitempty"`
}

// OpenAPIExample represents an example in the OpenAPI spec
type OpenAPIExample struct {
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
	Value       interface{} `json:"value,omitempty"`
}

// OpenAPISchema represents a schema in the OpenAPI spec
type OpenAPISchema struct {
	Ref                  string                    `json:"$ref,omitempty"`
	Type                 string                    `json:"type,omitempty"`
	Format               string                    `json:"format,omitempty"`
	Description          string                    `json:"description,omitempty"`
	Enum                 []interface{}             `json:"enum,omitempty"`
	Default              interface{}               `json:"default,omitempty"`
	Properties           map[string]*OpenAPISchema `json:"properties,omitempty"`
	Required             []string                  `json:"required,omitempty"`
	Items                *OpenAPISchema            `json:"items,omitempty"`
	AdditionalProperties *OpenAPISchema            `json:"additionalProperties,omitempty"`
	Nullable             bool                      `json:"nullable,omitempty"`
	Example              interface{}               `json:"example,omitempty"`
}

// OpenAPIComponents contains reusable components
type OpenAPIComponents struct {
	Schemas         map[string]*OpenAPISchema        `json:"schemas,omitempty"`
	Responses       map[string]OpenAPIResponse       `json:"responses,omitempty"`
	Parameters      map[string]OpenAPIParameter      `json:"parameters,omitempty"`
	RequestBodies   map[string]OpenAPIRequestBody    `json:"requestBodies,omitempty"`
	Headers         map[string]OpenAPIHeader         `json:"headers,omitempty"`
	SecuritySchemes map[string]OpenAPISecurityScheme `json:"securitySchemes,omitempty"`
	// ClickyFilters holds the reusable named-filter definitions referenced by
	// filter parameters' x-clicky-lookup.$ref, emitted under x-clicky-filters.
	ClickyFilters map[string]entity.FilterSpec `json:"x-clicky-filters,omitempty"`
}

// OpenAPISecurityScheme represents a security scheme
type OpenAPISecurityScheme struct {
	Type         string             `json:"type"`
	Description  string             `json:"description,omitempty"`
	Name         string             `json:"name,omitempty"`
	In           string             `json:"in,omitempty"`
	Scheme       string             `json:"scheme,omitempty"`
	BearerFormat string             `json:"bearerFormat,omitempty"`
	Flows        *OpenAPIOAuthFlows `json:"flows,omitempty"`
}

// OpenAPIOAuthFlows represents OAuth flows
type OpenAPIOAuthFlows struct {
	Implicit          *OpenAPIOAuthFlow `json:"implicit,omitempty"`
	Password          *OpenAPIOAuthFlow `json:"password,omitempty"`
	ClientCredentials *OpenAPIOAuthFlow `json:"clientCredentials,omitempty"`
	AuthorizationCode *OpenAPIOAuthFlow `json:"authorizationCode,omitempty"`
}

// OpenAPIOAuthFlow represents an OAuth flow
type OpenAPIOAuthFlow struct {
	AuthorizationURL string            `json:"authorizationUrl,omitempty"`
	TokenURL         string            `json:"tokenUrl,omitempty"`
	RefreshURL       string            `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes"`
}

// OpenAPITag represents a tag for grouping operations
type OpenAPITag struct {
	Name         string               `json:"name"`
	Description  string               `json:"description,omitempty"`
	ExternalDocs *OpenAPIExternalDocs `json:"externalDocs,omitempty"`
}

// OpenAPIExternalDocs represents external documentation
type OpenAPIExternalDocs struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

// OpenAPIGenerator generates OpenAPI specifications from RPC services
type OpenAPIGenerator struct {
	config     *OpenAPIConfig
	components *OpenAPIComponents // Shared components for reusable schemas
}

// OpenAPIConfig holds configuration for OpenAPI generation
type OpenAPIConfig struct {
	Title       string
	Description string
	Version     string
	Servers     []OpenAPIServer
	Contact     *OpenAPIContact
	License     *OpenAPILicense
	Tags        []OpenAPITag
}

// NewOpenAPIGenerator creates a new OpenAPI generator
func NewOpenAPIGenerator(config *OpenAPIConfig) *OpenAPIGenerator {
	if config == nil {
		config = &OpenAPIConfig{
			Title:       "API",
			Description: "Generated API from CLI commands",
			Version:     "1.0.0",
		}
	}
	return &OpenAPIGenerator{
		config: config,
		components: &OpenAPIComponents{
			Schemas: make(map[string]*OpenAPISchema),
		},
	}
}

// GenerateFromService generates an OpenAPI spec from an RPC service
func (g *OpenAPIGenerator) GenerateFromService(service *RPCService) *OpenAPISpec {
	spec := &OpenAPISpec{
		OpenAPI: "3.0.3",
		Info: OpenAPIInfo{
			Title:       g.config.Title,
			Description: g.config.Description,
			Version:     g.config.Version,
			Contact:     g.config.Contact,
			License:     g.config.License,
		},
		Servers: g.config.Servers,
		Paths:   make(map[string]OpenAPIPath),
		Tags:    g.config.Tags,
	}
	spec.Clicky = g.buildClickySpecMeta(service.Operations)

	// Group operations by path
	pathMap := make(map[string][]RPCOperation)
	for _, op := range service.Operations {
		path := op.Path
		if path == "" {
			// Generate path from operation name
			path = g.generatePathFromName(op.Name)
		}
		pathMap[path] = append(pathMap[path], op)
	}

	// Convert operations to OpenAPI paths
	for path, operations := range pathMap {
		openAPIPath := make(OpenAPIPath)

		for _, op := range operations {
			method := strings.ToLower(op.Method)
			if method == "" {
				method = "post"
			}

			openAPIPath[method] = g.convertOperationToOpenAPI(op)
		}

		spec.Paths[path] = openAPIPath
	}

	// Add components if any schemas or reusable filter definitions were generated
	if len(g.components.Schemas) > 0 || len(g.components.ClickyFilters) > 0 {
		spec.Components = g.components
	}

	return spec
}

// GenerateFromCobra generates an OpenAPI spec directly from a Cobra command tree
// using the default converter Config. Equivalent to GenerateFromCobraWithConfig
// with a nil config.
func (g *OpenAPIGenerator) GenerateFromCobra(rootCmd *cobra.Command) (*OpenAPISpec, error) {
	return g.GenerateFromCobraWithConfig(rootCmd, nil)
}

// GenerateFromCobraWithConfig generates an OpenAPI spec from a Cobra command
// tree using the supplied converter Config (path prefix, default method, tags).
// Pass nil to fall back to DefaultConfig. Callers that mount the executor under
// a non-default PathPrefix (see ExecutorConfig.PathPrefix) MUST use this so the
// generated spec paths match the actually-registered ServeMux patterns.
func (g *OpenAPIGenerator) GenerateFromCobraWithConfig(rootCmd *cobra.Command, cfg *Config) (*OpenAPISpec, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	converter := NewConverter(cfg)
	service, err := converter.ConvertCommandTree(rootCmd)
	if err != nil {
		return nil, fmt.Errorf("failed to convert commands to RPC service: %w", err)
	}

	return g.GenerateFromService(service), nil
}

// convertOperationToOpenAPI converts an RPC operation to an OpenAPI operation
func (g *OpenAPIGenerator) convertOperationToOpenAPI(op RPCOperation) OpenAPIOperation {
	openAPIOp := OpenAPIOperation{
		Tags:        op.Tags,
		Summary:     op.Description,
		Description: op.Description,
		OperationID: strings.ReplaceAll(op.Name, " ", "_"),
		Parameters:  []OpenAPIParameter{},
		Responses:   make(map[string]OpenAPIResponse),
	}
	if op.Clicky != nil && (op.Clicky.Surface != "" || op.Clicky.Command != "") {
		meta := *op.Clicky
		meta.SurfaceID = ""
		meta.Entity = ""
		meta.Parent = ""
		meta.Aliases = nil
		meta.Admin = false
		meta.Icon = ""
		meta.Title = ""
		meta.Order = 0
		openAPIOp.Clicky = &meta
	}

	// Convert parameters
	for _, param := range op.Parameters {
		openAPIParam := OpenAPIParameter{
			Name:        param.Name,
			In:          param.In,
			Description: param.Description,
			Required:    param.Required,
			Schema: g.convertPropertyToOpenAPI(Property{
				Type:        param.Type,
				Description: param.Description,
				Default:     param.Default,
			}),
		}
		if role := ParamRole(op, param); role != "" {
			openAPIParam.Clicky = &ClickyParameterMeta{Role: role}
			if role == "filter" {
				openAPIParam.Lookup = g.filterLookupMeta(op, param.Name)
			}
		}
		openAPIOp.Parameters = append(openAPIOp.Parameters, openAPIParam)
	}

	// Add request body for POST/PUT operations
	if op.Method == "POST" || op.Method == "PUT" {
		openAPIOp.RequestBody = &OpenAPIRequestBody{
			Description: "Request body",
			Content: map[string]OpenAPIMediaType{
				"application/json": {
					Schema: g.convertRPCSchemaToOpenAPI(op.Schema),
				},
			},
		}
	}

	// Add standard responses
	openAPIOp.Responses["200"] = OpenAPIResponse{
		Description: "Successful operation",
		Headers:     g.responseHeadersForOperation(op),
		Content: map[string]OpenAPIMediaType{
			"application/json": {
				Schema: g.responseSchemaForOperation(op),
			},
		},
	}

	openAPIOp.Responses["400"] = OpenAPIResponse{
		Description: "Bad Request",
		Content: map[string]OpenAPIMediaType{
			"application/json": {
				Schema: g.executionResponseSchema(),
			},
		},
	}

	openAPIOp.Responses["500"] = OpenAPIResponse{
		Description: "Internal Server Error",
		Content: map[string]OpenAPIMediaType{
			"application/json": {
				Schema: g.executionResponseSchema(),
			},
		},
	}

	return openAPIOp
}

// filterLookupMeta builds the x-clicky-lookup extension for a filter parameter
// that references a reusable named filter, registering the filter's definition
// into components.x-clicky-filters. Returns nil when the parameter is not backed
// by a named filter (a plain Filter[ListOpts] keeps the convention-only path).
func (g *OpenAPIGenerator) filterLookupMeta(op RPCOperation, paramName string) *ClickyLookupMeta {
	if op.Clicky == nil || op.Clicky.Entity == "" {
		return nil
	}
	info, ok := clicky.GetEntity(op.Clicky.Entity)
	if !ok {
		return nil
	}
	filterName, ok := info.FilterRefs[paramName]
	if !ok {
		return nil
	}
	nf, ok := entity.GetFilter(filterName)
	if !ok {
		return nil
	}

	spec := nf.Spec()
	if g.components.ClickyFilters == nil {
		g.components.ClickyFilters = make(map[string]entity.FilterSpec)
	}
	g.components.ClickyFilters[filterName] = spec

	return &ClickyLookupMeta{
		Ref:         "#/components/x-clicky-filters/" + filterName,
		URL:         op.Path,
		Filter:      paramName,
		SearchParam: "__lookup_q",
		Multi:       spec.Multi,
	}
}

// ParamRole classifies a parameter within its operation, deriving the
// supportsLookup/isListOp signals from the operation's Clicky metadata. Returns
// the UI role ("limit", "offset", "time-from", "time-to", "filter") or "".
func ParamRole(op RPCOperation, param RPCParameter) string {
	supportsLookup := op.Clicky != nil && op.Clicky.SupportsLookup
	isListOp := op.Clicky != nil && op.Clicky.Verb == "list"
	return paramRole(param, supportsLookup, isListOp)
}

// paramRole classifies a parameter so the UI can route it to the right widget
// (pagination footer, time-range picker, filter pill) instead of falling back
// to a generic text input. Returns "" when the parameter has no special role.
//
// Pagination roles ("limit", "offset") apply only on list operations because
// non-list ops that happen to take a literal `limit` flag would otherwise be
// hijacked. Time-range roles trigger on the conventional `since`/`from` and
// `to`/`until` names used across the codebase.
//
// "filter" is assigned to any remaining query-string parameter on a list op
// that declares SupportsLookup — that flag is the signal that the entity
// registered explicit Filter[ListOpts] entries, so every non-pagination query
// param maps to a filter chip. Non-query params (path/body/header) and params
// on non-lookup ops get no role.
func paramRole(param RPCParameter, supportsLookup, isListOp bool) string {
	if param.In != "query" {
		return ""
	}
	name := strings.ToLower(param.Name)
	if isListOp {
		switch name {
		case "limit":
			return "limit"
		case "offset":
			return "offset"
		case "since", "from":
			return "time-from"
		case "to", "until":
			return "time-to"
		}
	}
	if isListOp && supportsLookup {
		return "filter"
	}
	return ""
}

func (g *OpenAPIGenerator) executionResponseSchema() *OpenAPISchema {
	return g.convertGoTypeToOpenAPI(reflect.TypeOf(ExecutionResponse{}))
}

func (g *OpenAPIGenerator) responseSchemaForOperation(op RPCOperation) *OpenAPISchema {
	if op.ResponseType == nil {
		return &OpenAPISchema{
			Type: "object",
			Properties: map[string]*OpenAPISchema{
				"success": {
					Type:        "boolean",
					Description: "Operation success status",
				},
				"message": {
					Type:        "string",
					Description: "Operation result message",
				},
			},
		}
	}

	schema := g.convertGoTypeToOpenAPI(op.ResponseType)
	if op.ResponseEntityID {
		addEntityIDSchema(schema)
	}
	if op.ResponsePaged {
		return &OpenAPISchema{
			Type: "object",
			Properties: map[string]*OpenAPISchema{
				"data": {
					Type:  "array",
					Items: schema,
				},
				"page": pageInfoSchema(),
			},
			Required: []string{"data", "page"},
		}
	}
	if op.ResponseArray {
		schema = &OpenAPISchema{
			Type:  "array",
			Items: schema,
		}
	}
	return schema
}

func (g *OpenAPIGenerator) responseHeadersForOperation(op RPCOperation) map[string]OpenAPIHeader {
	if !op.ResponsePaged {
		return nil
	}
	integer := &OpenAPISchema{Type: "integer"}
	return map[string]OpenAPIHeader{
		"X-Total-Count": {
			Description: "Total number of matching rows before paging.",
			Schema:      integer,
		},
		"X-Page-Limit": {
			Description: "Effective page limit applied to the response.",
			Schema:      integer,
		},
		"X-Page-Offset": {
			Description: "Effective page offset applied to the response.",
			Schema:      integer,
		},
	}
}

func pageInfoSchema() *OpenAPISchema {
	return &OpenAPISchema{
		Type: "object",
		Properties: map[string]*OpenAPISchema{
			"limit": {
				Type:        "integer",
				Description: "Effective page limit.",
			},
			"offset": {
				Type:        "integer",
				Description: "Effective page offset.",
			},
			"total": {
				Type:        "integer",
				Description: "Total number of matching rows before paging.",
			},
		},
		Required: []string{"limit", "offset", "total"},
	}
}

func addEntityIDSchema(schema *OpenAPISchema) {
	if schema == nil {
		return
	}
	if schema.Type == "array" && schema.Items != nil {
		addEntityIDSchema(schema.Items)
		return
	}
	if schema.Type != "object" {
		return
	}
	if schema.Properties == nil {
		schema.Properties = map[string]*OpenAPISchema{}
	}
	schema.Properties["_id"] = &OpenAPISchema{
		Type:        "string",
		Description: "Clicky entity row identifier",
	}
}

func (g *OpenAPIGenerator) buildClickySpecMeta(operations []RPCOperation) *ClickySpecMeta {
	type surfaceEntry struct {
		surface ClickySurface
		baseKey string
	}

	if len(operations) == 0 {
		return nil
	}

	surfacesByID := make(map[string]*surfaceEntry)
	surfaceIDs := make([]string, 0)

	for index := range operations {
		meta := operations[index].Clicky
		if meta == nil || meta.SurfaceID == "" || meta.Entity == "" {
			continue
		}

		meta.Order = index
		entry := surfacesByID[meta.SurfaceID]
		if entry == nil {
			baseKey := clickySurfaceBaseKey(meta.Entity, meta.Aliases)
			title := meta.Title
			if title == "" {
				title = clickySurfaceTitle(baseKey, meta.Admin)
			}
			entry = &surfaceEntry{
				surface: ClickySurface{
					Entity: meta.Entity,
					Parent: meta.Parent,
					Admin:  meta.Admin,
					Title:  title,
					Icon:   meta.Icon,
					Order:  index,
				},
				baseKey: baseKey,
			}
			surfacesByID[meta.SurfaceID] = entry
			surfaceIDs = append(surfaceIDs, meta.SurfaceID)
		}

		if entry.surface.Description == "" && meta.Verb == "list" && operations[index].Description != "" {
			entry.surface.Description = operations[index].Description
		}
	}

	if len(surfaceIDs) == 0 {
		return nil
	}

	candidateCounts := make(map[string]int)
	for _, id := range surfaceIDs {
		entry := surfacesByID[id]
		candidateCounts[clickySurfaceCandidateKey(entry.baseKey, entry.surface.Parent, entry.surface.Admin)]++
	}

	surfaces := make([]ClickySurface, 0, len(surfaceIDs))
	resolvedKeys := make(map[string]string, len(surfaceIDs))
	for _, id := range surfaceIDs {
		entry := surfacesByID[id]
		key := clickySurfaceCandidateKey(entry.baseKey, entry.surface.Parent, entry.surface.Admin)
		if candidateCounts[key] > 1 && entry.surface.Parent != "" {
			key = entry.surface.Parent + "-" + key
		}
		entry.surface.Key = key
		if entry.surface.Description == "" {
			entry.surface.Description = fmt.Sprintf("Manage %s resources.", strings.ToLower(entry.surface.Title))
		}
		resolvedKeys[id] = key
		surfaces = append(surfaces, entry.surface)
	}

	for index := range operations {
		if operations[index].Clicky == nil {
			continue
		}
		operations[index].Clicky.Surface = resolvedKeys[operations[index].Clicky.SurfaceID]
	}

	sort.SliceStable(surfaces, func(i, j int) bool {
		if surfaces[i].Admin != surfaces[j].Admin {
			return !surfaces[i].Admin
		}
		return surfaces[i].Order < surfaces[j].Order
	})

	return &ClickySpecMeta{Surfaces: surfaces}
}

func clickySurfaceBaseKey(entity string, aliases []string) string {
	return clickyPluralize(entity)
}

func clickySurfaceCandidateKey(baseKey string, parent string, admin bool) string {
	if admin {
		return "admin-" + baseKey
	}
	return baseKey
}

func clickySurfaceTitle(baseKey string, admin bool) string {
	title := clickyTitleCase(baseKey)
	if admin {
		return "Admin — " + title
	}
	return title
}

func clickyPluralize(value string) string {
	switch {
	case value == "":
		return value
	case strings.HasSuffix(value, "s"):
		return value
	case strings.HasSuffix(value, "x"), strings.HasSuffix(value, "z"),
		strings.HasSuffix(value, "ch"), strings.HasSuffix(value, "sh"):
		return value + "es"
	default:
		return value + "s"
	}
}

func clickyTitleCase(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for index, part := range parts {
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

// convertRPCSchemaToOpenAPI converts an RPC schema to OpenAPI schema
func (g *OpenAPIGenerator) convertRPCSchemaToOpenAPI(schema Schema) *OpenAPISchema {
	openAPISchema := &OpenAPISchema{
		Type:       schema.Type,
		Properties: make(map[string]*OpenAPISchema),
		Required:   schema.Required,
	}

	for name, prop := range schema.Properties {
		openAPISchema.Properties[name] = g.convertPropertyToOpenAPI(prop)
	}

	return openAPISchema
}

// convertPropertyToOpenAPI converts a Property to OpenAPI schema with enhanced type handling
func (g *OpenAPIGenerator) convertPropertyToOpenAPI(prop Property) *OpenAPISchema {
	schema := &OpenAPISchema{
		Type:        prop.Type,
		Description: prop.Description,
		Default:     prop.Default,
	}

	if len(prop.Enum) > 0 {
		schema.Enum = make([]interface{}, len(prop.Enum))
		for i, v := range prop.Enum {
			schema.Enum[i] = v
		}
	}

	// Handle different types with enhanced logic
	switch prop.Type {
	case "string":
		g.enhanceStringSchema(schema, prop)
	case "integer", "int", "int32", "int64":
		g.enhanceIntegerSchema(schema, prop)
	case "number", "float", "float32", "float64":
		g.enhanceNumberSchema(schema, prop)
	case "array":
		g.enhanceArraySchema(schema, prop)
	case "object", "struct":
		g.enhanceObjectSchema(schema, prop)
	case "boolean":
		schema.Type = "boolean"
	}

	return schema
}

// enhanceStringSchema adds format and validation for string types
func (g *OpenAPIGenerator) enhanceStringSchema(schema *OpenAPISchema, prop Property) {
	schema.Type = "string"
	descLower := strings.ToLower(prop.Description)

	if strings.Contains(descLower, "email") {
		schema.Format = "email"
	} else if strings.Contains(descLower, "uri") || strings.Contains(descLower, "url") {
		schema.Format = "uri"
	} else if strings.Contains(descLower, "uuid") || strings.Contains(descLower, "guid") {
		schema.Format = "uuid"
	} else if strings.Contains(descLower, "password") {
		schema.Format = "password"
	} else if strings.Contains(descLower, "date") {
		if strings.Contains(descLower, "time") {
			schema.Format = "date-time"
		} else {
			schema.Format = "date"
		}
	} else if strings.Contains(descLower, "binary") || strings.Contains(descLower, "base64") {
		schema.Format = "byte"
	}
}

// enhanceIntegerSchema adds format for integer types
func (g *OpenAPIGenerator) enhanceIntegerSchema(schema *OpenAPISchema, prop Property) {
	schema.Type = "integer"
	descLower := strings.ToLower(prop.Description)

	if strings.Contains(descLower, "timestamp") || prop.Type == "int64" {
		schema.Format = "int64"
	} else {
		schema.Format = "int32"
	}
}

// enhanceNumberSchema adds format for number types
func (g *OpenAPIGenerator) enhanceNumberSchema(schema *OpenAPISchema, prop Property) {
	schema.Type = "number"
	if prop.Type == "float32" {
		schema.Format = "float"
	} else {
		schema.Format = "double"
	}
}

// enhanceArraySchema handles array types with items schema
func (g *OpenAPIGenerator) enhanceArraySchema(schema *OpenAPISchema, prop Property) {
	schema.Type = "array"
	// For now, default to string items - this could be enhanced to parse item types from descriptions
	schema.Items = &OpenAPISchema{
		Type: "string",
	}

	// Try to infer item type from description
	descLower := strings.ToLower(prop.Description)
	if strings.Contains(descLower, "array of integers") || strings.Contains(descLower, "list of numbers") {
		schema.Items.Type = "integer"
	} else if strings.Contains(descLower, "array of objects") || strings.Contains(descLower, "list of structs") {
		schema.Items.Type = "object"
	}
}

// enhanceObjectSchema handles object/struct types
func (g *OpenAPIGenerator) enhanceObjectSchema(schema *OpenAPISchema, prop Property) {
	schema.Type = "object"
	// Additional properties could be added here if needed
	schema.Properties = make(map[string]*OpenAPISchema)
}

// generatePathFromName generates a REST path from an operation name
func (g *OpenAPIGenerator) generatePathFromName(name string) string {
	// Convert "user create" to "/user"
	// Convert "config set" to "/config"
	parts := strings.Split(name, " ")

	if len(parts) == 0 {
		return "/api"
	}

	// Use the first part as the resource, skip CRUD verbs
	resource := parts[0]

	// Use resource name as-is without pluralization
	// Keep the original resource name from the command
	return "/" + resource
}

// ToJSON converts the OpenAPI spec to JSON
func (spec *OpenAPISpec) ToJSON() ([]byte, error) {
	return json.MarshalIndent(spec, "", "  ")
}

// ToYAML converts the OpenAPI spec to YAML
func (spec *OpenAPISpec) ToYAML() ([]byte, error) {
	return yaml.Marshal(spec)
}

// GenerateFromRPCService creates an OpenAPI spec directly from an RPC service
func GenerateFromRPCService(service *RPCService, config *OpenAPIConfig) (*OpenAPISpec, error) {
	generator := NewOpenAPIGenerator(config)
	return generator.GenerateFromService(service), nil
}

// GenerateFromRPCOperation creates an OpenAPI spec from a single RPC operation
func GenerateFromRPCOperation(operation *RPCOperation, config *OpenAPIConfig) (*OpenAPISpec, error) {
	service := &RPCService{
		Name:        "single-operation",
		Version:     "1.0.0",
		Description: "Generated from single RPC operation",
		Operations:  []RPCOperation{*operation},
	}
	return GenerateFromRPCService(service, config)
}
