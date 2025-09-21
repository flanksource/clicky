package rpc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// OpenAPISpec represents an OpenAPI 3.0 specification
type OpenAPISpec struct {
	OpenAPI    string                 `json:"openapi"`
	Info       OpenAPIInfo            `json:"info"`
	Servers    []OpenAPIServer        `json:"servers,omitempty"`
	Paths      map[string]OpenAPIPath `json:"paths"`
	Components *OpenAPIComponents     `json:"components,omitempty"`
	Tags       []OpenAPITag           `json:"tags,omitempty"`
}

// OpenAPIInfo contains metadata about the API
type OpenAPIInfo struct {
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Version     string         `json:"version"`
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
	URL         string                    `json:"url"`
	Description string                    `json:"description,omitempty"`
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
	Tags        []string                       `json:"tags,omitempty"`
	Summary     string                         `json:"summary,omitempty"`
	Description string                         `json:"description,omitempty"`
	OperationID string                         `json:"operationId,omitempty"`
	Parameters  []OpenAPIParameter             `json:"parameters,omitempty"`
	RequestBody *OpenAPIRequestBody            `json:"requestBody,omitempty"`
	Responses   map[string]OpenAPIResponse     `json:"responses"`
	Security    []map[string][]string          `json:"security,omitempty"`
}

// OpenAPIParameter represents a parameter in the OpenAPI spec
type OpenAPIParameter struct {
	Name        string             `json:"name"`
	In          string             `json:"in"`
	Description string             `json:"description,omitempty"`
	Required    bool               `json:"required,omitempty"`
	Schema      *OpenAPISchema     `json:"schema,omitempty"`
	Example     interface{}        `json:"example,omitempty"`
}

// OpenAPIRequestBody represents a request body in the OpenAPI spec
type OpenAPIRequestBody struct {
	Description string                        `json:"description,omitempty"`
	Content     map[string]OpenAPIMediaType   `json:"content"`
	Required    bool                          `json:"required,omitempty"`
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
	Schema   *OpenAPISchema `json:"schema,omitempty"`
	Example  interface{}    `json:"example,omitempty"`
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
	Type        string                    `json:"type,omitempty"`
	Format      string                    `json:"format,omitempty"`
	Description string                    `json:"description,omitempty"`
	Enum        []interface{}             `json:"enum,omitempty"`
	Default     interface{}               `json:"default,omitempty"`
	Properties  map[string]*OpenAPISchema `json:"properties,omitempty"`
	Required    []string                  `json:"required,omitempty"`
	Items       *OpenAPISchema            `json:"items,omitempty"`
	Example     interface{}               `json:"example,omitempty"`
}

// OpenAPIComponents contains reusable components
type OpenAPIComponents struct {
	Schemas         map[string]*OpenAPISchema   `json:"schemas,omitempty"`
	Responses       map[string]OpenAPIResponse  `json:"responses,omitempty"`
	Parameters      map[string]OpenAPIParameter `json:"parameters,omitempty"`
	RequestBodies   map[string]OpenAPIRequestBody `json:"requestBodies,omitempty"`
	Headers         map[string]OpenAPIHeader    `json:"headers,omitempty"`
	SecuritySchemes map[string]OpenAPISecurityScheme `json:"securitySchemes,omitempty"`
}

// OpenAPISecurityScheme represents a security scheme
type OpenAPISecurityScheme struct {
	Type         string            `json:"type"`
	Description  string            `json:"description,omitempty"`
	Name         string            `json:"name,omitempty"`
	In           string            `json:"in,omitempty"`
	Scheme       string            `json:"scheme,omitempty"`
	BearerFormat string            `json:"bearerFormat,omitempty"`
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
	config *OpenAPIConfig
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
	return &OpenAPIGenerator{config: config}
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

	return spec
}

// GenerateFromCobra generates an OpenAPI spec directly from a Cobra command tree
func (g *OpenAPIGenerator) GenerateFromCobra(rootCmd *cobra.Command) (*OpenAPISpec, error) {
	converter := NewConverter(DefaultConfig())
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

	// Convert parameters
	for _, param := range op.Parameters {
		openAPIParam := OpenAPIParameter{
			Name:        param.Name,
			In:          param.In,
			Description: param.Description,
			Required:    param.Required,
			Schema:      g.convertSchemaToOpenAPI(Property{
				Type:        param.Type,
				Description: param.Description,
				Default:     param.Default,
			}),
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
		Content: map[string]OpenAPIMediaType{
			"application/json": {
				Schema: &OpenAPISchema{
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
				},
			},
		},
	}

	openAPIOp.Responses["400"] = OpenAPIResponse{
		Description: "Bad Request",
		Content: map[string]OpenAPIMediaType{
			"application/json": {
				Schema: &OpenAPISchema{
					Type: "object",
					Properties: map[string]*OpenAPISchema{
						"error": {
							Type:        "string",
							Description: "Error message",
						},
					},
				},
			},
		},
	}

	openAPIOp.Responses["500"] = OpenAPIResponse{
		Description: "Internal Server Error",
		Content: map[string]OpenAPIMediaType{
			"application/json": {
				Schema: &OpenAPISchema{
					Type: "object",
					Properties: map[string]*OpenAPISchema{
						"error": {
							Type:        "string",
							Description: "Error message",
						},
					},
				},
			},
		},
	}

	return openAPIOp
}

// convertRPCSchemaToOpenAPI converts an RPC schema to OpenAPI schema
func (g *OpenAPIGenerator) convertRPCSchemaToOpenAPI(schema Schema) *OpenAPISchema {
	openAPISchema := &OpenAPISchema{
		Type:       schema.Type,
		Properties: make(map[string]*OpenAPISchema),
		Required:   schema.Required,
	}

	for name, prop := range schema.Properties {
		openAPISchema.Properties[name] = g.convertSchemaToOpenAPI(prop)
	}

	return openAPISchema
}

// convertSchemaToOpenAPI converts a Property to OpenAPI schema
func (g *OpenAPIGenerator) convertSchemaToOpenAPI(prop Property) *OpenAPISchema {
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

	// Add format hints for common types
	switch prop.Type {
	case "string":
		if strings.Contains(strings.ToLower(prop.Description), "email") {
			schema.Format = "email"
		} else if strings.Contains(strings.ToLower(prop.Description), "uri") ||
			strings.Contains(strings.ToLower(prop.Description), "url") {
			schema.Format = "uri"
		} else if strings.Contains(strings.ToLower(prop.Description), "date") {
			schema.Format = "date-time"
		}
	case "integer":
		if strings.Contains(strings.ToLower(prop.Description), "timestamp") {
			schema.Format = "int64"
		} else {
			schema.Format = "int32"
		}
	case "number":
		schema.Format = "double"
	}

	return schema
}

// generatePathFromName generates a REST path from an operation name
func (g *OpenAPIGenerator) generatePathFromName(name string) string {
	// Convert "user create" to "/users"
	// Convert "config set" to "/config"
	parts := strings.Split(name, " ")

	if len(parts) == 0 {
		return "/api"
	}

	// Use the first part as the resource, skip CRUD verbs
	resource := parts[0]

	// Simple pluralization for REST convention
	if !strings.HasSuffix(resource, "s") {
		resource += "s"
	}

	return "/" + resource
}

// ToJSON converts the OpenAPI spec to JSON
func (spec *OpenAPISpec) ToJSON() ([]byte, error) {
	return json.MarshalIndent(spec, "", "  ")
}

// ToYAML converts the OpenAPI spec to YAML (would need yaml package)
func (spec *OpenAPISpec) ToYAML() ([]byte, error) {
	// Would need to implement YAML marshaling
	// For now, return JSON as a placeholder
	return spec.ToJSON()
}