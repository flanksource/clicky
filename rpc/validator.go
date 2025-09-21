package rpc

import (
	"fmt"
	"strings"
)

// OpenAPIValidator provides validation for OpenAPI specifications
type OpenAPIValidator struct{}

// NewOpenAPIValidator creates a new OpenAPI validator
func NewOpenAPIValidator() *OpenAPIValidator {
	return &OpenAPIValidator{}
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult contains the results of validation
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// Validate validates an OpenAPI specification
func (v *OpenAPIValidator) Validate(spec *OpenAPISpec) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	// Validate required top-level fields
	v.validateInfo(spec, result)
	v.validateOpenAPIVersion(spec, result)
	v.validatePaths(spec, result)

	// If any errors were found, mark as invalid
	if len(result.Errors) > 0 {
		result.Valid = false
	}

	return result
}

// validateInfo validates the info section
func (v *OpenAPIValidator) validateInfo(spec *OpenAPISpec, result *ValidationResult) {
	if spec.Info.Title == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "info.title",
			Message: "title is required",
		})
	}

	if spec.Info.Version == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "info.version",
			Message: "version is required",
		})
	}

	// Validate version format (basic semantic versioning check)
	if spec.Info.Version != "" && !v.isValidVersion(spec.Info.Version) {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "info.version",
			Message: "version should follow semantic versioning format (e.g., 1.0.0)",
		})
	}
}

// validateOpenAPIVersion validates the OpenAPI version
func (v *OpenAPIValidator) validateOpenAPIVersion(spec *OpenAPISpec, result *ValidationResult) {
	if spec.OpenAPI == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "openapi",
			Message: "openapi version is required",
		})
		return
	}

	// Check for supported versions
	supportedVersions := []string{"3.0.0", "3.0.1", "3.0.2", "3.0.3", "3.1.0"}
	if !v.contains(supportedVersions, spec.OpenAPI) {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "openapi",
			Message: fmt.Sprintf("unsupported OpenAPI version: %s (supported: %s)", spec.OpenAPI, strings.Join(supportedVersions, ", ")),
		})
	}
}

// validatePaths validates the paths section
func (v *OpenAPIValidator) validatePaths(spec *OpenAPISpec, result *ValidationResult) {
	if len(spec.Paths) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "paths",
			Message: "at least one path is required",
		})
		return
	}

	for path, pathItem := range spec.Paths {
		v.validatePath(path, pathItem, result)
	}
}

// validatePath validates a single path
func (v *OpenAPIValidator) validatePath(path string, pathItem OpenAPIPath, result *ValidationResult) {
	// Validate path format
	if !strings.HasPrefix(path, "/") {
		result.Errors = append(result.Errors, ValidationError{
			Field:   fmt.Sprintf("paths[%s]", path),
			Message: "path must start with '/'",
		})
	}

	// Validate that at least one operation exists
	if len(pathItem) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   fmt.Sprintf("paths[%s]", path),
			Message: "path must have at least one operation",
		})
		return
	}

	// Validate each operation
	for method, operation := range pathItem {
		v.validateOperation(path, method, operation, result)
	}
}

// validateOperation validates a single operation
func (v *OpenAPIValidator) validateOperation(path, method string, operation OpenAPIOperation, result *ValidationResult) {
	fieldPrefix := fmt.Sprintf("paths[%s].%s", path, method)

	// Validate HTTP method
	validMethods := []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}
	if !v.contains(validMethods, strings.ToLower(method)) {
		result.Errors = append(result.Errors, ValidationError{
			Field:   fieldPrefix,
			Message: fmt.Sprintf("invalid HTTP method: %s", method),
		})
	}

	// Validate responses (at least one is required)
	if len(operation.Responses) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   fmt.Sprintf("%s.responses", fieldPrefix),
			Message: "at least one response is required",
		})
	}

	// Validate response codes
	for responseCode := range operation.Responses {
		if !v.isValidResponseCode(responseCode) {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("%s.responses[%s]", fieldPrefix, responseCode),
				Message: fmt.Sprintf("invalid response code: %s", responseCode),
			})
		}
	}

	// Validate parameters
	for i, param := range operation.Parameters {
		v.validateParameter(fmt.Sprintf("%s.parameters[%d]", fieldPrefix, i), param, result)
	}

	// Validate operation ID uniqueness (basic check)
	if operation.OperationID != "" && !v.isValidOperationID(operation.OperationID) {
		result.Errors = append(result.Errors, ValidationError{
			Field:   fmt.Sprintf("%s.operationId", fieldPrefix),
			Message: "operationId should contain only letters, numbers, and underscores",
		})
	}
}

// validateParameter validates a parameter
func (v *OpenAPIValidator) validateParameter(fieldPrefix string, param OpenAPIParameter, result *ValidationResult) {
	if param.Name == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   fmt.Sprintf("%s.name", fieldPrefix),
			Message: "parameter name is required",
		})
	}

	if param.In == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   fmt.Sprintf("%s.in", fieldPrefix),
			Message: "parameter 'in' is required",
		})
	} else {
		validIns := []string{"query", "header", "path", "cookie"}
		if !v.contains(validIns, param.In) {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("%s.in", fieldPrefix),
				Message: fmt.Sprintf("invalid parameter location: %s (valid: %s)", param.In, strings.Join(validIns, ", ")),
			})
		}
	}

	// Path parameters must be required
	if param.In == "path" && !param.Required {
		result.Errors = append(result.Errors, ValidationError{
			Field:   fmt.Sprintf("%s.required", fieldPrefix),
			Message: "path parameters must be required",
		})
	}
}

// Helper functions

// isValidVersion checks if a version string is valid (basic semantic versioning)
func (v *OpenAPIValidator) isValidVersion(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}
	// Basic check - should have at least major.minor
	return len(parts) >= 2 && len(parts) <= 3
}

// isValidResponseCode checks if a response code is valid
func (v *OpenAPIValidator) isValidResponseCode(code string) bool {
	// Allow standard HTTP status codes and 'default'
	if code == "default" {
		return true
	}

	// Check if it's a valid HTTP status code pattern
	if len(code) != 3 {
		return false
	}

	// Check if first digit is 1-5 (1xx, 2xx, 3xx, 4xx, 5xx)
	if code[0] < '1' || code[0] > '5' {
		return false
	}

	// Check if remaining digits are numeric or 'X'
	for i := 1; i < 3; i++ {
		if code[i] != 'X' && code[i] != 'x' && (code[i] < '0' || code[i] > '9') {
			return false
		}
	}

	return true
}

// isValidOperationID checks if an operation ID is valid
func (v *OpenAPIValidator) isValidOperationID(operationID string) bool {
	if operationID == "" {
		return false
	}

	for _, char := range operationID {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			 (char >= '0' && char <= '9') || char == '_') {
			return false
		}
	}

	return true
}

// contains checks if a slice contains a string
func (v *OpenAPIValidator) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ValidateRPCService validates an RPC service before converting to OpenAPI
func ValidateRPCService(service *RPCService) error {
	if service == nil {
		return fmt.Errorf("service cannot be nil")
	}

	if service.Name == "" {
		return fmt.Errorf("service name is required")
	}

	if service.Version == "" {
		return fmt.Errorf("service version is required")
	}

	if len(service.Operations) == 0 {
		return fmt.Errorf("service must have at least one operation")
	}

	// Validate each operation
	for i, op := range service.Operations {
		if err := ValidateRPCOperation(&op); err != nil {
			return fmt.Errorf("operation %d (%s): %w", i, op.Name, err)
		}
	}

	return nil
}

// ValidateRPCOperation validates a single RPC operation
func ValidateRPCOperation(operation *RPCOperation) error {
	if operation == nil {
		return fmt.Errorf("operation cannot be nil")
	}

	if operation.Name == "" {
		return fmt.Errorf("operation name is required")
	}

	if operation.Description == "" {
		return fmt.Errorf("operation description is required")
	}

	// Validate HTTP method if specified
	if operation.Method != "" {
		validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
		method := strings.ToUpper(operation.Method)
		valid := false
		for _, validMethod := range validMethods {
			if method == validMethod {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid HTTP method: %s", operation.Method)
		}
	}

	// Validate path format if specified
	if operation.Path != "" && !strings.HasPrefix(operation.Path, "/") {
		return fmt.Errorf("path must start with '/': %s", operation.Path)
	}

	return nil
}