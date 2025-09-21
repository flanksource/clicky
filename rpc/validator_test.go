package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAPIValidator_Validate(t *testing.T) {
	validator := NewOpenAPIValidator()

	tests := []struct {
		name      string
		spec      *OpenAPISpec
		wantValid bool
		wantErrors []string
	}{
		{
			name: "valid minimal spec",
			spec: &OpenAPISpec{
				OpenAPI: "3.0.3",
				Info: OpenAPIInfo{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]OpenAPIPath{
					"/users": {
						"get": OpenAPIOperation{
							Responses: map[string]OpenAPIResponse{
								"200": {Description: "Success"},
							},
						},
					},
				},
			},
			wantValid:  true,
			wantErrors: []string{},
		},
		{
			name: "missing required fields",
			spec: &OpenAPISpec{
				OpenAPI: "",
				Info: OpenAPIInfo{
					Title:   "",
					Version: "",
				},
				Paths: map[string]OpenAPIPath{},
			},
			wantValid: false,
			wantErrors: []string{
				"info.title: title is required",
				"info.version: version is required",
				"openapi: openapi version is required",
				"paths: at least one path is required",
			},
		},
		{
			name: "invalid version format",
			spec: &OpenAPISpec{
				OpenAPI: "3.0.3",
				Info: OpenAPIInfo{
					Title:   "Test API",
					Version: "invalid",
				},
				Paths: map[string]OpenAPIPath{
					"/users": {
						"get": OpenAPIOperation{
							Responses: map[string]OpenAPIResponse{
								"200": {Description: "Success"},
							},
						},
					},
				},
			},
			wantValid: false,
			wantErrors: []string{
				"info.version: version should follow semantic versioning format (e.g., 1.0.0)",
			},
		},
		{
			name: "unsupported OpenAPI version",
			spec: &OpenAPISpec{
				OpenAPI: "2.0",
				Info: OpenAPIInfo{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]OpenAPIPath{
					"/users": {
						"get": OpenAPIOperation{
							Responses: map[string]OpenAPIResponse{
								"200": {Description: "Success"},
							},
						},
					},
				},
			},
			wantValid: false,
			wantErrors: []string{
				"openapi: unsupported OpenAPI version: 2.0 (supported: 3.0.0, 3.0.1, 3.0.2, 3.0.3, 3.1.0)",
			},
		},
		{
			name: "invalid path format",
			spec: &OpenAPISpec{
				OpenAPI: "3.0.3",
				Info: OpenAPIInfo{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]OpenAPIPath{
					"users": {
						"get": OpenAPIOperation{
							Responses: map[string]OpenAPIResponse{
								"200": {Description: "Success"},
							},
						},
					},
				},
			},
			wantValid: false,
			wantErrors: []string{
				"paths[users]: path must start with '/'",
			},
		},
		{
			name: "invalid HTTP method",
			spec: &OpenAPISpec{
				OpenAPI: "3.0.3",
				Info: OpenAPIInfo{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]OpenAPIPath{
					"/users": {
						"invalid": OpenAPIOperation{
							Responses: map[string]OpenAPIResponse{
								"200": {Description: "Success"},
							},
						},
					},
				},
			},
			wantValid: false,
			wantErrors: []string{
				"paths[/users].invalid: invalid HTTP method: invalid",
			},
		},
		{
			name: "missing responses",
			spec: &OpenAPISpec{
				OpenAPI: "3.0.3",
				Info: OpenAPIInfo{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]OpenAPIPath{
					"/users": {
						"get": OpenAPIOperation{
							Responses: map[string]OpenAPIResponse{},
						},
					},
				},
			},
			wantValid: false,
			wantErrors: []string{
				"paths[/users].get.responses: at least one response is required",
			},
		},
		{
			name: "invalid response code",
			spec: &OpenAPISpec{
				OpenAPI: "3.0.3",
				Info: OpenAPIInfo{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]OpenAPIPath{
					"/users": {
						"get": OpenAPIOperation{
							Responses: map[string]OpenAPIResponse{
								"999": {Description: "Invalid"},
							},
						},
					},
				},
			},
			wantValid: false,
			wantErrors: []string{
				"paths[/users].get.responses[999]: invalid response code: 999",
			},
		},
		{
			name: "invalid parameter",
			spec: &OpenAPISpec{
				OpenAPI: "3.0.3",
				Info: OpenAPIInfo{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]OpenAPIPath{
					"/users": {
						"get": OpenAPIOperation{
							Parameters: []OpenAPIParameter{
								{
									Name: "",
									In:   "invalid",
								},
							},
							Responses: map[string]OpenAPIResponse{
								"200": {Description: "Success"},
							},
						},
					},
				},
			},
			wantValid: false,
			wantErrors: []string{
				"paths[/users].get.parameters[0].name: parameter name is required",
				"paths[/users].get.parameters[0].in: invalid parameter location: invalid (valid: query, header, path, cookie)",
			},
		},
		{
			name: "path parameter not required",
			spec: &OpenAPISpec{
				OpenAPI: "3.0.3",
				Info: OpenAPIInfo{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]OpenAPIPath{
					"/users/{id}": {
						"get": OpenAPIOperation{
							Parameters: []OpenAPIParameter{
								{
									Name:     "id",
									In:       "path",
									Required: false,
								},
							},
							Responses: map[string]OpenAPIResponse{
								"200": {Description: "Success"},
							},
						},
					},
				},
			},
			wantValid: false,
			wantErrors: []string{
				"paths[/users/{id}].get.parameters[0].required: path parameters must be required",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.spec)
			assert.Equal(t, tt.wantValid, result.Valid)

			if len(tt.wantErrors) > 0 {
				require.Len(t, result.Errors, len(tt.wantErrors))
				for i, expectedError := range tt.wantErrors {
					assert.Equal(t, expectedError, result.Errors[i].Error())
				}
			} else {
				assert.Empty(t, result.Errors)
			}
		})
	}
}

func TestOpenAPIValidator_isValidVersion(t *testing.T) {
	validator := NewOpenAPIValidator()

	tests := []struct {
		version string
		want    bool
	}{
		{"1.0.0", true},
		{"1.0", true},
		{"2.1.3", true},
		{"10.5.2", true},
		{"1", false},
		{"", false},
		{"invalid", false},
		{"1.0.0.0", false}, // 4 parts not allowed in basic version check
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := validator.isValidVersion(tt.version)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOpenAPIValidator_isValidResponseCode(t *testing.T) {
	validator := NewOpenAPIValidator()

	tests := []struct {
		code string
		want bool
	}{
		{"200", true},
		{"404", true},
		{"500", true},
		{"2XX", true},
		{"4xx", true},
		{"default", true},
		{"999", false}, // Invalid first digit
		{"99", false},  // Too short
		{"1234", false}, // Too long
		{"abc", false},  // Non-numeric
		{"", false},     // Empty
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := validator.isValidResponseCode(tt.code)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOpenAPIValidator_isValidOperationID(t *testing.T) {
	validator := NewOpenAPIValidator()

	tests := []struct {
		operationID string
		want        bool
	}{
		{"getUserById", true},
		{"user_create", true},
		{"list123", true},
		{"valid_operation_ID", true},
		{"", false},
		{"user-create", false}, // hyphen not allowed
		{"user create", false}, // space not allowed
		{"user@create", false}, // special chars not allowed
	}

	for _, tt := range tests {
		t.Run(tt.operationID, func(t *testing.T) {
			got := validator.isValidOperationID(tt.operationID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateRPCService(t *testing.T) {
	tests := []struct {
		name        string
		service     *RPCService
		wantError   bool
		errorSubstr string
	}{
		{
			name:        "nil service",
			service:     nil,
			wantError:   true,
			errorSubstr: "service cannot be nil",
		},
		{
			name: "missing name",
			service: &RPCService{
				Name:       "",
				Version:    "1.0.0",
				Operations: []RPCOperation{},
			},
			wantError:   true,
			errorSubstr: "service name is required",
		},
		{
			name: "missing version",
			service: &RPCService{
				Name:       "test-service",
				Version:    "",
				Operations: []RPCOperation{},
			},
			wantError:   true,
			errorSubstr: "service version is required",
		},
		{
			name: "no operations",
			service: &RPCService{
				Name:       "test-service",
				Version:    "1.0.0",
				Operations: []RPCOperation{},
			},
			wantError:   true,
			errorSubstr: "service must have at least one operation",
		},
		{
			name: "valid service",
			service: &RPCService{
				Name:    "test-service",
				Version: "1.0.0",
				Operations: []RPCOperation{
					{
						Name:        "test operation",
						Description: "Test operation",
						Method:      "GET",
						Path:        "/test",
					},
				},
			},
			wantError: false,
		},
		{
			name: "invalid operation",
			service: &RPCService{
				Name:    "test-service",
				Version: "1.0.0",
				Operations: []RPCOperation{
					{
						Name:        "",
						Description: "Test operation",
					},
				},
			},
			wantError:   true,
			errorSubstr: "operation name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRPCService(tt.service)
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorSubstr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRPCOperation(t *testing.T) {
	tests := []struct {
		name        string
		operation   *RPCOperation
		wantError   bool
		errorSubstr string
	}{
		{
			name:        "nil operation",
			operation:   nil,
			wantError:   true,
			errorSubstr: "operation cannot be nil",
		},
		{
			name: "missing name",
			operation: &RPCOperation{
				Name:        "",
				Description: "Test",
			},
			wantError:   true,
			errorSubstr: "operation name is required",
		},
		{
			name: "missing description",
			operation: &RPCOperation{
				Name:        "test",
				Description: "",
			},
			wantError:   true,
			errorSubstr: "operation description is required",
		},
		{
			name: "invalid HTTP method",
			operation: &RPCOperation{
				Name:        "test",
				Description: "Test operation",
				Method:      "INVALID",
			},
			wantError:   true,
			errorSubstr: "invalid HTTP method: INVALID",
		},
		{
			name: "invalid path format",
			operation: &RPCOperation{
				Name:        "test",
				Description: "Test operation",
				Path:        "invalid-path",
			},
			wantError:   true,
			errorSubstr: "path must start with '/': invalid-path",
		},
		{
			name: "valid operation",
			operation: &RPCOperation{
				Name:        "test",
				Description: "Test operation",
				Method:      "GET",
				Path:        "/test",
			},
			wantError: false,
		},
		{
			name: "valid operation with case insensitive method",
			operation: &RPCOperation{
				Name:        "test",
				Description: "Test operation",
				Method:      "post",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRPCOperation(tt.operation)
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorSubstr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}