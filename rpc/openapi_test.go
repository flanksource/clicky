package rpc

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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