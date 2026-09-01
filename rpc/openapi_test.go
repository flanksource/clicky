package rpc

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/flanksource/clicky"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenAPIGenerator_DynamicEntitySurfaceIcon asserts that the x-clicky-icon /
// x-clicky-path / x-clicky-title carried by a dynamic entity propagate to the
// OpenAPI surface (x-clicky.surfaces[].icon / .path / .title), and that icon
// does NOT leak onto the operation-level x-clicky.
func TestOpenAPIGenerator_DynamicEntitySurfaceIcon(t *testing.T) {
	const entityName = "openapi-icon-stack"
	root := &cobra.Command{Use: "testapp"}

	clicky.RegisterDynamicEntity(clicky.DynamicEntitySpec{
		Name:     entityName,
		Icon:     "database",
		Path:     "jms/incoming",
		Title:    "My Stack",
		ListType: reflect.StructOf(nil),
		List: func(context.Context, map[string]string, []string) (any, error) {
			return []map[string]any{}, nil
		},
	})

	clicky.GenerateCLI(root)

	spec, err := NewOpenAPIGenerator(nil).GenerateFromCobra(root)
	require.NoError(t, err)
	require.NotNil(t, spec.Clicky, "spec must carry x-clicky surfaces")

	var surface *ClickySurface
	for i := range spec.Clicky.Surfaces {
		if spec.Clicky.Surfaces[i].Entity == entityName {
			surface = &spec.Clicky.Surfaces[i]
			break
		}
	}
	require.NotNil(t, surface, "surface for %q missing", entityName)
	assert.Equal(t, "database", surface.Icon, "surface carries the entity icon")
	assert.Equal(t, "jms/incoming", surface.Path, "surface carries the entity hierarchy path")
	assert.Equal(t, "My Stack", surface.Title, "surface title overridden by x-clicky-title")

	listOp := spec.Paths["/api/v1/"+entityName]["get"]
	require.NotNil(t, listOp.Clicky, "list operation should carry x-clicky meta")
	assert.Empty(t, listOp.Clicky.Icon, "icon must stay surface-only, not on the operation")
}

// TestOpenAPIGenerator_StaticEntitySurfacePath asserts a statically registered
// entity can declare a hierarchy path too: the surface metadata is the frontend's
// only source of it, so an entity that cannot state one is invisible in the tree
// no matter where it belongs.
func TestOpenAPIGenerator_StaticEntitySurfacePath(t *testing.T) {
	const entityName = "openapi-path-stack"
	root := &cobra.Command{Use: "testapp"}

	clicky.NewEntity[openAPISchemaListItem, openAPISchemaOpts, openAPISchemaDetail](entityName).
		Path("jms", "incoming").
		List(func(openAPISchemaOpts) ([]openAPISchemaListItem, error) {
			return nil, nil
		}).
		Register()

	clicky.GenerateCLI(root)

	spec, err := NewOpenAPIGenerator(nil).GenerateFromCobra(root)
	require.NoError(t, err)
	require.NotNil(t, spec.Clicky, "spec must carry x-clicky surfaces")

	var surface *ClickySurface
	for i := range spec.Clicky.Surfaces {
		if spec.Clicky.Surfaces[i].Entity == entityName {
			surface = &spec.Clicky.Surfaces[i]
			break
		}
	}
	require.NotNil(t, surface, "surface for %q missing", entityName)
	assert.Equal(t, "jms/incoming", surface.Path,
		"a static entity's path reaches the surface, as a dynamic entity's does")
}

// TestOpenAPIGenerator_SurfaceKeyNotPluralized asserts the surface route key is
// the singular entity name (no automatic pluralization), and that the same key
// is stamped on the list and get operations. clicky-ui builds a row-click route
// of /<x-clicky.surface>/<id> and resolves it against x-clicky.surfaces[].key,
// so spec-key == per-op-surface == /api/v1/<name> path segment must all match
// for the navigation to resolve. The name ends in a consonant, where the old
// naive pluralizer would have appended "s".
func TestOpenAPIGenerator_SurfaceKeyNotPluralized(t *testing.T) {
	const entityName = "openapi-policy-stack"
	root := &cobra.Command{Use: "testapp"}

	clicky.NewEntity[openAPISchemaListItem, openAPISchemaOpts, openAPISchemaDetail](entityName).
		List(func(openAPISchemaOpts) ([]openAPISchemaListItem, error) {
			return nil, nil
		}).
		Get(func(id string) (openAPISchemaDetail, error) {
			return openAPISchemaDetail{ID: id, Name: id}, nil
		}).
		Register()

	clicky.GenerateCLI(root)

	spec, err := NewOpenAPIGenerator(nil).GenerateFromCobra(root)
	require.NoError(t, err)
	require.NotNil(t, spec.Clicky, "spec must carry x-clicky surfaces")

	var surface *ClickySurface
	for i := range spec.Clicky.Surfaces {
		if spec.Clicky.Surfaces[i].Entity == entityName {
			surface = &spec.Clicky.Surfaces[i]
			break
		}
	}
	require.NotNil(t, surface, "surface for %q missing", entityName)
	assert.Equal(t, entityName, surface.Key,
		"surface key must be the singular entity name, not pluralized")

	listOp := spec.Paths["/api/v1/"+entityName]["get"]
	require.NotNil(t, listOp.Clicky, "list operation should carry x-clicky meta")
	assert.Equal(t, entityName, listOp.Clicky.Surface,
		"list op surface must equal the singular key (matches the /api/v1/<name> path)")

	getOp := spec.Paths["/api/v1/"+entityName+"/{id}"]["get"]
	require.NotNil(t, getOp.Clicky, "get operation should carry x-clicky meta")
	assert.Equal(t, entityName, getOp.Clicky.Surface,
		"get op surface must equal the singular key so the row-click route resolves")
}

func TestOpenAPIGenerator_NewOpenAPIGenerator(t *testing.T) {
	tests := []struct {
		name   string
		config *OpenAPIConfig
		want   *OpenAPIGenerator
	}{
		{
			name:   "with nil config",
			config: nil,
			want: &OpenAPIGenerator{
				config: &OpenAPIConfig{
					Title:       "API",
					Description: "Generated API from CLI commands",
					Version:     "1.0.0",
				},
			},
		},
		{
			name: "with custom config",
			config: &OpenAPIConfig{
				Title:       "Test API",
				Description: "Test Description",
				Version:     "2.0.0",
			},
			want: &OpenAPIGenerator{
				config: &OpenAPIConfig{
					Title:       "Test API",
					Description: "Test Description",
					Version:     "2.0.0",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewOpenAPIGenerator(tt.config)
			assert.Equal(t, tt.want.config.Title, got.config.Title)
			assert.Equal(t, tt.want.config.Description, got.config.Description)
			assert.Equal(t, tt.want.config.Version, got.config.Version)
			assert.NotNil(t, got.components)
			assert.NotNil(t, got.components.Schemas)
		})
	}
}

func TestOpenAPIGenerator_ConvertPropertyToOpenAPI(t *testing.T) {
	generator := NewOpenAPIGenerator(nil)

	tests := []struct {
		name     string
		property Property
		want     *OpenAPISchema
	}{
		{
			name: "string with email description",
			property: Property{
				Type:        "string",
				Description: "User email address",
				Default:     "test@example.com",
			},
			want: &OpenAPISchema{
				Type:        "string",
				Description: "User email address",
				Default:     "test@example.com",
				Format:      "email",
			},
		},
		{
			name: "string with URL description",
			property: Property{
				Type:        "string",
				Description: "Profile URL",
			},
			want: &OpenAPISchema{
				Type:        "string",
				Description: "Profile URL",
				Format:      "uri",
			},
		},
		{
			name: "string with UUID description",
			property: Property{
				Type:        "string",
				Description: "User UUID identifier",
			},
			want: &OpenAPISchema{
				Type:        "string",
				Description: "User UUID identifier",
				Format:      "uuid",
			},
		},
		{
			name: "string with date-time description",
			property: Property{
				Type:        "string",
				Description: "Created date time",
			},
			want: &OpenAPISchema{
				Type:        "string",
				Description: "Created date time",
				Format:      "date-time",
			},
		},
		{
			name: "integer with timestamp description",
			property: Property{
				Type:        "integer",
				Description: "Unix timestamp",
			},
			want: &OpenAPISchema{
				Type:        "integer",
				Description: "Unix timestamp",
				Format:      "int64",
			},
		},
		{
			name: "integer without special description",
			property: Property{
				Type:        "integer",
				Description: "User age",
			},
			want: &OpenAPISchema{
				Type:        "integer",
				Description: "User age",
				Format:      "int32",
			},
		},
		{
			name: "number float32",
			property: Property{
				Type:        "float32",
				Description: "Score percentage",
			},
			want: &OpenAPISchema{
				Type:        "number",
				Description: "Score percentage",
				Format:      "float",
			},
		},
		{
			name: "number float64",
			property: Property{
				Type:        "float64",
				Description: "Precise calculation",
			},
			want: &OpenAPISchema{
				Type:        "number",
				Description: "Precise calculation",
				Format:      "double",
			},
		},
		{
			name: "array type",
			property: Property{
				Type:        "array",
				Description: "List of tags",
			},
			want: &OpenAPISchema{
				Type:        "array",
				Description: "List of tags",
				Items: &OpenAPISchema{
					Type: "string",
				},
			},
		},
		{
			name: "array of integers",
			property: Property{
				Type:        "array",
				Description: "Array of integers for IDs",
			},
			want: &OpenAPISchema{
				Type:        "array",
				Description: "Array of integers for IDs",
				Items: &OpenAPISchema{
					Type: "integer",
				},
			},
		},
		{
			name: "object/struct type",
			property: Property{
				Type:        "struct",
				Description: "User profile data",
			},
			want: &OpenAPISchema{
				Type:        "object",
				Description: "User profile data",
				Properties:  map[string]*OpenAPISchema{},
			},
		},
		{
			name: "boolean type",
			property: Property{
				Type:        "boolean",
				Description: "Is active",
			},
			want: &OpenAPISchema{
				Type:        "boolean",
				Description: "Is active",
			},
		},
		{
			name: "enum property",
			property: Property{
				Type:        "string",
				Description: "Status value",
				Enum:        []string{"active", "inactive", "pending"},
			},
			want: &OpenAPISchema{
				Type:        "string",
				Description: "Status value",
				Enum:        []interface{}{"active", "inactive", "pending"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generator.convertPropertyToOpenAPI(tt.property)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.Description, got.Description)
			assert.Equal(t, tt.want.Format, got.Format)
			assert.Equal(t, tt.want.Default, got.Default)
			assert.Equal(t, tt.want.Enum, got.Enum)

			if tt.want.Items != nil {
				require.NotNil(t, got.Items)
				assert.Equal(t, tt.want.Items.Type, got.Items.Type)
			}

			if tt.want.Properties != nil {
				assert.NotNil(t, got.Properties)
			}
		})
	}
}

func TestOpenAPIGenerator_GenerateFromService(t *testing.T) {
	generator := NewOpenAPIGenerator(&OpenAPIConfig{
		Title:       "Test API",
		Description: "Test API Description",
		Version:     "1.0.0",
	})

	service := &RPCService{
		Name:        "test-service",
		Version:     "1.0.0",
		Description: "Test service",
		Operations: []RPCOperation{
			{
				Name:        "user create",
				Description: "Create a new user",
				Method:      "POST",
				Path:        "/users",
				Parameters: []RPCParameter{
					{
						Name:        "name",
						Type:        "string",
						Description: "User name",
						Required:    true,
						In:          "query",
					},
					{
						Name:        "email",
						Type:        "string",
						Description: "User email address",
						Required:    true,
						In:          "query",
					},
				},
				Schema: Schema{
					Type: "object",
					Properties: map[string]Property{
						"name": {
							Type:        "string",
							Description: "User name",
						},
						"email": {
							Type:        "string",
							Description: "User email address",
						},
					},
					Required: []string{"name", "email"},
				},
				Tags: []string{"users"},
			},
			{
				Name:        "user list",
				Description: "List all users",
				Method:      "GET",
				Path:        "/users",
				Parameters: []RPCParameter{
					{
						Name:        "limit",
						Type:        "integer",
						Description: "Maximum number of users to return",
						Required:    false,
						In:          "query",
						Default:     10,
					},
				},
				Schema: Schema{
					Type:       "object",
					Properties: map[string]Property{},
					Required:   []string{},
				},
				Tags: []string{"users"},
			},
		},
	}

	spec := generator.GenerateFromService(service)

	// Test basic spec structure
	assert.Equal(t, "3.0.3", spec.OpenAPI)
	assert.Equal(t, "Test API", spec.Info.Title)
	assert.Equal(t, "Test API Description", spec.Info.Description)
	assert.Equal(t, "1.0.0", spec.Info.Version)

	// Test paths
	assert.Contains(t, spec.Paths, "/users")
	usersPath := spec.Paths["/users"]

	// Test POST operation
	assert.Contains(t, usersPath, "post")
	postOp := usersPath["post"]
	assert.Equal(t, []string{"users"}, postOp.Tags)
	assert.Equal(t, "Create a new user", postOp.Summary)
	assert.Len(t, postOp.Parameters, 2)
	assert.NotNil(t, postOp.RequestBody)

	// Test GET operation
	assert.Contains(t, usersPath, "get")
	getOp := usersPath["get"]
	assert.Equal(t, []string{"users"}, getOp.Tags)
	assert.Equal(t, "List all users", getOp.Summary)
	assert.Len(t, getOp.Parameters, 1)
	assert.Equal(t, "limit", getOp.Parameters[0].Name)
	assert.Equal(t, 10, getOp.Parameters[0].Schema.Default)

	// Test responses
	assert.Contains(t, postOp.Responses, "200")
	assert.Contains(t, postOp.Responses, "400")
	assert.Contains(t, postOp.Responses, "500")
}

func TestOpenAPIGenerator_GenerateFromCobra(t *testing.T) {
	// Create a test Cobra command tree
	rootCmd := &cobra.Command{
		Use:   "testapp",
		Short: "Test application",
		Long:  "A test application for OpenAPI generation",
	}

	userCmd := &cobra.Command{
		Use:   "user",
		Short: "User management commands",
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	createCmd.Flags().String("name", "", "User name")
	createCmd.Flags().String("email", "", "User email")
	createCmd.Flags().Bool("admin", false, "Is admin user")
	createCmd.Flags().Int("age", 0, "User age")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	listCmd.Flags().Int("limit", 10, "Maximum number of users")
	listCmd.Flags().String("sort", "name", "Sort field")

	userCmd.AddCommand(createCmd, listCmd)
	rootCmd.AddCommand(userCmd)

	generator := NewOpenAPIGenerator(&OpenAPIConfig{
		Title:       "Test App API",
		Description: "Generated from test app commands",
		Version:     "1.0.0",
	})

	spec, err := generator.GenerateFromCobra(rootCmd)
	require.NoError(t, err)

	// Test basic structure
	assert.Equal(t, "3.0.3", spec.OpenAPI)
	assert.Equal(t, "Test App API", spec.Info.Title)
	assert.Equal(t, "Generated from test app commands", spec.Info.Description)
	assert.Equal(t, "1.0.0", spec.Info.Version)

	// Test that operations were generated
	assert.NotEmpty(t, spec.Paths)

	// The exact paths depend on the path generation logic,
	// but we should have operations for user create and user list
	foundCreate := false
	foundList := false

	for path, pathItem := range spec.Paths {
		for _, operation := range pathItem {
			if strings.Contains(operation.OperationID, "user_create") {
				foundCreate = true
				// Test create operation has the right parameters
				assert.Contains(t, getParameterNames(operation.Parameters), "name")
				assert.Contains(t, getParameterNames(operation.Parameters), "email")
				assert.Contains(t, getParameterNames(operation.Parameters), "admin")
				assert.Contains(t, getParameterNames(operation.Parameters), "age")
			}
			if strings.Contains(operation.OperationID, "user_list") {
				foundList = true
				// Test list operation has the right parameters
				assert.Contains(t, getParameterNames(operation.Parameters), "limit")
				assert.Contains(t, getParameterNames(operation.Parameters), "sort")
			}
		}
		t.Logf("Path: %s, Methods: %v", path, getMethodNames(pathItem))
	}

	assert.True(t, foundCreate, "Should find user create operation")
	assert.True(t, foundList, "Should find user list operation")
}

type openAPISchemaListItem struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	CreatedAt time.Time         `json:"created_at"`
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Hidden    string            `json:"-"`
}

func (i openAPISchemaListItem) GetID() string   { return i.ID }
func (i openAPISchemaListItem) GetName() string { return i.Name }

type openAPISchemaDetail struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Optional *string `json:"optional,omitempty"`
	Secret   string  `pretty:"hide"`
}

type openAPIRestartResult struct {
	Restarted bool `json:"restarted"`
}

type openAPIPauseResult struct {
	Count int `json:"count"`
}

type openAPISchemaOpts struct{}

func TestOpenAPIGenerator_EntityResponseSchemas(t *testing.T) {
	const entityName = "openapi-schema-stack"
	root := &cobra.Command{Use: "testapp"}

	clicky.NewEntity[openAPISchemaListItem, openAPISchemaOpts, openAPISchemaDetail](entityName).
		List(func(openAPISchemaOpts) ([]openAPISchemaListItem, error) {
			return nil, nil
		}).
		Get(func(id string) (openAPISchemaDetail, error) {
			return openAPISchemaDetail{ID: id, Name: id}, nil
		}).
		WithAction(clicky.Action("restart", func(id string, flags map[string]string) (openAPIRestartResult, error) {
			return openAPIRestartResult{Restarted: true}, nil
		})).
		WithBulkAction(clicky.BulkAction("pause", func(ids []string, flags map[string]string) (openAPIPauseResult, error) {
			return openAPIPauseResult{Count: len(ids)}, nil
		})).
		Register()

	clicky.GenerateCLI(root)

	spec, err := NewOpenAPIGenerator(nil).GenerateFromCobra(root)
	require.NoError(t, err)

	listOp := spec.Paths["/api/v1/"+entityName]["get"]
	listSchema := requireResponseSchema(t, listOp)
	require.Equal(t, "array", listSchema.Type)
	require.NotNil(t, listSchema.Items)
	assert.Equal(t, "object", listSchema.Items.Type)
	assert.Contains(t, listSchema.Items.Properties, "_id")
	assert.Contains(t, listSchema.Items.Properties, "created_at")
	assert.Equal(t, "date-time", listSchema.Items.Properties["created_at"].Format)
	assert.Equal(t, "array", listSchema.Items.Properties["tags"].Type)
	assert.Equal(t, "object", listSchema.Items.Properties["metadata"].Type)
	assert.NotContains(t, listSchema.Items.Properties, "Hidden")
	assert.NotContains(t, listSchema.Items.Properties, "hidden")

	getOp := spec.Paths["/api/v1/"+entityName+"/{id}"]["get"]
	getSchema := requireResponseSchema(t, getOp)
	assert.Contains(t, getSchema.Properties, "optional")
	assert.True(t, getSchema.Properties["optional"].Nullable)
	assert.NotContains(t, getSchema.Properties, "secret")

	restartOp := spec.Paths["/api/v1/"+entityName+"/{id}/restart"]["post"]
	restartSchema := requireResponseSchema(t, restartOp)
	assert.Contains(t, restartSchema.Properties, "restarted")

	pauseOp := spec.Paths["/api/v1/"+entityName+"/pause"]["post"]
	pauseSchema := requireResponseSchema(t, pauseOp)
	assert.Contains(t, pauseSchema.Properties, "count")
}

// TestOpenAPIGenerator_OptionalIDActionPath asserts that an entity action
// declared WithOptionalID generates a flat /api/v1/<entity>/<action> path
// (no {id} segment), so a no-id call does not collide with the entity's
// get-by-id route. A regular action without WithOptionalID keeps the
// /{id}/<action> shape.
func TestOpenAPIGenerator_OptionalIDActionPath(t *testing.T) {
	const entityName = "openapi-optional-id-stack"
	root := &cobra.Command{Use: "testapp"}

	clicky.NewEntity[openAPISchemaListItem, openAPISchemaOpts, openAPISchemaDetail](entityName).
		List(func(openAPISchemaOpts) ([]openAPISchemaListItem, error) {
			return nil, nil
		}).
		Get(func(id string) (openAPISchemaDetail, error) {
			return openAPISchemaDetail{ID: id, Name: id}, nil
		}).
		WithAction(clicky.Action("overview", func(string, map[string]string) (openAPIPauseResult, error) {
			return openAPIPauseResult{}, nil
		}).WithMethod("GET").WithOptionalID()).
		WithAction(clicky.Action("restart", func(string, map[string]string) (openAPIRestartResult, error) {
			return openAPIRestartResult{Restarted: true}, nil
		})).
		Register()

	clicky.GenerateCLI(root)

	spec, err := NewOpenAPIGenerator(nil).GenerateFromCobra(root)
	require.NoError(t, err)

	flatPath := "/api/v1/" + entityName + "/overview"
	idScopedPath := "/api/v1/" + entityName + "/{id}/overview"
	assert.Contains(t, spec.Paths, flatPath,
		"optional-id action must register a flat path with no {id} segment")
	assert.NotContains(t, spec.Paths, idScopedPath,
		"optional-id action must NOT register an {id}-scoped path")
	assert.Contains(t, spec.Paths[flatPath], "get",
		"overview WithMethod(GET) must register under the GET verb")
	overview := spec.Paths[flatPath]["get"]
	require.NotNil(t, overview.Clicky)
	assert.Equal(t, "collection", overview.Clicky.Scope,
		"an action invocable without an entity id must be discoverable as a collection action")

	assert.Contains(t, spec.Paths, "/api/v1/"+entityName+"/{id}/restart",
		"a regular action without WithOptionalID keeps its {id} segment")
}

func requireResponseSchema(t *testing.T, op OpenAPIOperation) *OpenAPISchema {
	t.Helper()
	response, ok := op.Responses["200"]
	require.True(t, ok, "200 response missing")
	media, ok := response.Content["application/json"]
	require.True(t, ok, "application/json response missing")
	require.NotNil(t, media.Schema)
	return media.Schema
}

func TestOpenAPISpec_ToJSON(t *testing.T) {
	spec := &OpenAPISpec{
		OpenAPI: "3.0.3",
		Info: OpenAPIInfo{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Paths: make(map[string]OpenAPIPath),
	}

	data, err := spec.ToJSON()
	require.NoError(t, err)

	// Test that it's valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "3.0.3", parsed["openapi"])
	assert.Equal(t, "Test API", parsed["info"].(map[string]interface{})["title"])
	assert.Equal(t, "1.0.0", parsed["info"].(map[string]interface{})["version"])
}

func TestOpenAPISpec_ToYAML(t *testing.T) {
	spec := &OpenAPISpec{
		OpenAPI: "3.0.3",
		Info: OpenAPIInfo{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Paths: make(map[string]OpenAPIPath),
	}

	data, err := spec.ToYAML()
	require.NoError(t, err)

	// Test that it contains expected YAML content
	yamlStr := string(data)
	assert.Contains(t, yamlStr, "openapi: 3.0.3")
	assert.Contains(t, yamlStr, "title: Test API")
	assert.Contains(t, yamlStr, "version: 1.0.0")
}

func TestGenerateFromRPCService(t *testing.T) {
	service := &RPCService{
		Name:        "test-service",
		Version:     "1.0.0",
		Description: "Test service",
		Operations: []RPCOperation{
			{
				Name:        "test operation",
				Description: "Test operation description",
				Method:      "GET",
				Path:        "/test",
				Parameters:  []RPCParameter{},
				Schema: Schema{
					Type:       "object",
					Properties: map[string]Property{},
					Required:   []string{},
				},
			},
		},
	}

	config := &OpenAPIConfig{
		Title:       "Test API",
		Description: "Test Description",
		Version:     "1.0.0",
	}

	spec, err := GenerateFromRPCService(service, config)
	require.NoError(t, err)
	assert.NotNil(t, spec)
	assert.Equal(t, "Test API", spec.Info.Title)
	assert.Contains(t, spec.Paths, "/test")
}

func TestGenerateFromRPCOperation(t *testing.T) {
	operation := &RPCOperation{
		Name:        "test operation",
		Description: "Test operation description",
		Method:      "POST",
		Path:        "/test",
		Parameters: []RPCParameter{
			{
				Name:        "param1",
				Type:        "string",
				Description: "Test parameter",
				Required:    true,
				In:          "query",
			},
		},
		Schema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"param1": {
					Type:        "string",
					Description: "Test parameter",
				},
			},
			Required: []string{"param1"},
		},
	}

	config := &OpenAPIConfig{
		Title:       "Single Operation API",
		Description: "Generated from single operation",
		Version:     "1.0.0",
	}

	spec, err := GenerateFromRPCOperation(operation, config)
	require.NoError(t, err)
	assert.NotNil(t, spec)
	assert.Equal(t, "Single Operation API", spec.Info.Title)
	assert.Contains(t, spec.Paths, "/test")

	testPath := spec.Paths["/test"]
	assert.Contains(t, testPath, "post")

	postOp := testPath["post"]
	assert.Equal(t, "Test operation description", postOp.Summary)
	assert.Len(t, postOp.Parameters, 1)
	assert.Equal(t, "param1", postOp.Parameters[0].Name)
}

// Helper functions

func getParameterNames(params []OpenAPIParameter) []string {
	names := make([]string, len(params))
	for i, param := range params {
		names[i] = param.Name
	}
	return names
}

func getMethodNames(pathItem OpenAPIPath) []string {
	var methods []string
	for method := range pathItem {
		methods = append(methods, method)
	}
	return methods
}

func TestParamRole(t *testing.T) {
	tests := []struct {
		name           string
		param          RPCParameter
		supportsLookup bool
		isListOp       bool
		want           string
	}{
		{
			name:     "limit query on list op",
			param:    RPCParameter{Name: "limit", In: "query"},
			isListOp: true,
			want:     "limit",
		},
		{
			name:     "offset query on list op",
			param:    RPCParameter{Name: "offset", In: "query"},
			isListOp: true,
			want:     "offset",
		},
		{
			name:     "since query on list op",
			param:    RPCParameter{Name: "since", In: "query"},
			isListOp: true,
			want:     "time-from",
		},
		{
			name:     "from query on list op",
			param:    RPCParameter{Name: "from", In: "query"},
			isListOp: true,
			want:     "time-from",
		},
		{
			name:     "to query on list op",
			param:    RPCParameter{Name: "to", In: "query"},
			isListOp: true,
			want:     "time-to",
		},
		{
			name:     "until query on list op",
			param:    RPCParameter{Name: "until", In: "query"},
			isListOp: true,
			want:     "time-to",
		},
		{
			name:           "named query on lookup list op becomes filter",
			param:          RPCParameter{Name: "name", In: "query"},
			supportsLookup: true,
			isListOp:       true,
			want:           "filter",
		},
		{
			name:     "named query on list op without lookup gets no role",
			param:    RPCParameter{Name: "name", In: "query"},
			isListOp: true,
			want:     "",
		},
		{
			name:           "named query on non-list op with lookup gets no role",
			param:          RPCParameter{Name: "name", In: "query"},
			supportsLookup: true,
			want:           "",
		},
		{
			name:     "limit query on non-list op gets no role",
			param:    RPCParameter{Name: "limit", In: "query"},
			isListOp: false,
			want:     "",
		},
		{
			name:           "path parameter gets no role even when filterable name",
			param:          RPCParameter{Name: "name", In: "path"},
			supportsLookup: true,
			isListOp:       true,
			want:           "",
		},
		{
			name:           "body parameter gets no role",
			param:          RPCParameter{Name: "limit", In: "body"},
			supportsLookup: true,
			isListOp:       true,
			want:           "",
		},
		{
			name:     "uppercase LIMIT still detected",
			param:    RPCParameter{Name: "LIMIT", In: "query"},
			isListOp: true,
			want:     "limit",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := paramRole(tt.param, tt.supportsLookup, tt.isListOp)
			assert.Equal(t, tt.want, got)
		})
	}
}
