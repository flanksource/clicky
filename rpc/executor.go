package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// CommandExecutor handles dynamic execution of Cobra commands via HTTP requests
type CommandExecutor struct {
	service    *RPCService              // Pre-converted command tree
	operations map[string]*RPCOperation // path+method -> operation lookup
	config     *ExecutorConfig          // Execution configuration
}

// ExecutionRequest represents parameters for command execution
type ExecutionRequest struct {
	Args  []string          `json:"args,omitempty"`  // Positional arguments
	Flags map[string]string `json:"flags,omitempty"` // Flag values

	// Context carries the request's context.Context to a ContextDataFunc. On the
	// HTTP path it is r.Context(); on the CLI path it is cmd.Context(). It is
	// never serialized — it is transport state, not part of the wire contract.
	Context context.Context `json:"-"`
}

// ctx returns the request context, falling back to context.Background() when
// the request was built without one (older callers, tests).
func (r *ExecutionRequest) ctx() context.Context {
	if r != nil && r.Context != nil {
		return r.Context
	}
	return context.Background()
}

// ExecutionResponse represents the result of command execution
type ExecutionResponse struct {
	Success  bool              `json:"success"`
	Message  string            `json:"message,omitempty"`
	Output   string            `json:"output,omitempty"` // Combined stdout+stderr for backward compatibility
	Stdout   string            `json:"stdout,omitempty"` // Standard output only
	Stderr   string            `json:"stderr,omitempty"` // Standard error only
	ExitCode int               `json:"exit_code"`        // Command exit code (0 = success)
	Error    string            `json:"error,omitempty"`  // Error description
	Input    *ExecutionRequest `json:"input,omitempty"`  // Processed input parameters (included on errors for debugging)
	CLI      string            `json:"cli,omitempty"`    // Equivalent CLI command to reproduce this execution

	// DataIsStructured is true when the operation returned data via a DataFunc
	// (entity list/get) rather than captured stdout. The HTTP layer uses it to
	// serialize the data payload directly for structured wire formats instead of
	// substituting this (output-less) envelope. Not serialized — it's transport
	// metadata, not part of the response contract.
	DataIsStructured bool `json:"-"`
}

// NewCommandExecutor creates a new command executor
func NewCommandExecutor(service *RPCService, config *ExecutorConfig) *CommandExecutor {
	if config == nil {
		config = &ExecutorConfig{
			Enabled:    false,
			SkipPreRun: true,
			PathPrefix: "/api/v1",
		}
	}

	// Build operation lookup map
	operations := make(map[string]*RPCOperation)
	for i := range service.Operations {
		op := &service.Operations[i]
		// Create lookup key: method:path
		key := strings.ToUpper(op.Method) + ":" + op.Path
		operations[key] = op
	}

	return &CommandExecutor{
		service:    service,
		operations: operations,
		config:     config,
	}
}

// FindOperation finds an RPC operation by HTTP method and path.
// Supports templated paths like /api/v1/policy/{id} matched against
// concrete paths like /api/v1/policy/12345.
func (e *CommandExecutor) FindOperation(method, path string) *RPCOperation {
	return e.findOperationForMethod(method, path)
}

func (e *CommandExecutor) findOperationForMethod(method, path string) *RPCOperation {
	key := strings.ToUpper(method) + ":" + path
	if op := e.operations[key]; op != nil {
		return op
	}

	for _, op := range e.operations {
		if !strings.EqualFold(op.Method, method) {
			continue
		}
		if matchTemplatePath(op.Path, path) {
			return op
		}
	}

	return nil
}

func (e *CommandExecutor) FindLookupOperation(method, path string) *RPCOperation {
	if strings.EqualFold(method, http.MethodHead) {
		if op := e.findLookupOperationForMethod(http.MethodGet, path); op != nil {
			return op
		}
	}

	for _, candidateMethod := range []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	} {
		if op := e.findLookupOperationForMethod(candidateMethod, path); op != nil {
			return op
		}
	}

	return nil
}

func (e *CommandExecutor) findLookupOperationForMethod(method, path string) *RPCOperation {
	preferredMethods := []string{
		method,
	}

	for _, preferredMethod := range preferredMethods {
		for i := range e.service.Operations {
			op := &e.service.Operations[i]
			if !hasLookup(op) || !strings.EqualFold(op.Method, preferredMethod) {
				continue
			}
			if matchTemplatePath(op.Path, path) {
				return op
			}
		}
	}

	return nil
}

// matchTemplatePath checks if a concrete path matches a templated path pattern.
// e.g., "/api/v1/policy/{id}" matches "/api/v1/policy/12345"
// e.g., "/api/v1/policy/{id}/recalculate" matches "/api/v1/policy/12345/recalculate"
func matchTemplatePath(template, concrete string) bool {
	tParts := strings.Split(strings.Trim(template, "/"), "/")
	cParts := strings.Split(strings.Trim(concrete, "/"), "/")
	if len(tParts) != len(cParts) {
		return false
	}
	for i, t := range tParts {
		if strings.HasPrefix(t, "{") && strings.HasSuffix(t, "}") {
			continue // template segment matches anything
		}
		if t != cParts[i] {
			return false
		}
	}
	return true
}

// extractPathParams extracts parameter values from a concrete path using the template.
// e.g., template="/api/v1/policy/{id}", path="/api/v1/policy/12345" → {"id": "12345"}
func extractPathParams(template, path string) map[string]string {
	result := make(map[string]string)
	tParts := strings.Split(strings.Trim(template, "/"), "/")
	pParts := strings.Split(strings.Trim(path, "/"), "/")
	for i, t := range tParts {
		if i >= len(pParts) {
			break
		}
		if strings.HasPrefix(t, "{") && strings.HasSuffix(t, "}") {
			name := t[1 : len(t)-1]
			result[name] = pParts[i]
		}
	}
	return result
}

// ExtractRequestFromHTTP extracts execution parameters from HTTP request
func (e *CommandExecutor) ExtractRequestFromHTTP(r *http.Request, op *RPCOperation) (*ExecutionRequest, error) {
	// Buffer the body once so the raw (nested) JSON stays available to
	// context-based entity handlers via RequestFromContext, even though the
	// flag-flattening below also reads it. The internal decode reads from
	// bodyBytes; r.Body is reset to a fresh reader for the handler.
	var bodyBytes []byte
	if r.Body != nil {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}
		_ = r.Body.Close()
		bodyBytes = b
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	req := &ExecutionRequest{
		Flags:   make(map[string]string),
		Context: ContextWithRequest(r.Context(), r),
	}

	// Extract path parameters from URL using the template.
	// Path param values may be comma-delimited for bulk operations.
	pathParams := extractPathParams(op.Path, r.URL.Path)
	for _, param := range op.Parameters {
		if param.In == "path" {
			if value, ok := pathParams[param.Name]; ok {
				// Split comma-delimited IDs into separate args
				for _, id := range strings.Split(value, ",") {
					req.Args = append(req.Args, strings.TrimSpace(id))
				}
				req.Flags[param.Name] = value
			}
		}
	}

	// Extract JSON body for POST/PUT requests FIRST (so query params can override).
	// Parse the media type so a charset/boundary param (e.g. "application/json;
	// charset=utf-8") still matches.
	if r.Method == "POST" || r.Method == "PUT" {
		mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if mediaType == "application/json" && len(bodyBytes) > 0 {
			var bodyData map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &bodyData); err != nil {
				return nil, fmt.Errorf("failed to parse JSON body: %w", err)
			}

			// Handle flat JSON structure where each field is a flag or args
			for key, value := range bodyData {
				switch key {
				case "args":
					// Convert args to []string
					if argsArray, ok := value.([]interface{}); ok {
						for _, arg := range argsArray {
							req.Args = append(req.Args, fmt.Sprintf("%v", arg))
						}
					} else if argStr, ok := value.(string); ok {
						// Single string argument
						req.Args = append(req.Args, argStr)
					}
				case "flags":
					// Handle nested flags structure for backward compatibility
					if flagsMap, ok := value.(map[string]interface{}); ok {
						for flagName, flagValue := range flagsMap {
							req.Flags[flagName] = convertValueToString(flagValue)
						}
					}
				default:
					// Treat all other fields as flags
					req.Flags[key] = convertValueToString(value)
				}
			}
		}
	}

	// Extract query parameters LAST (so they take precedence over JSON body)
	// First, handle "args" specially since it's a common parameter
	if r.URL.Query().Has("args") {
		value := r.URL.Query().Get("args")
		// Special handling for args parameter - goes to Args array, not Flags map
		req.Args = []string{} // Clear existing args for precedence
		if value != "" {
			// Handle comma-separated values or single value
			req.Args = strings.Split(value, ",")
			// Trim whitespace from each arg
			for i := range req.Args {
				req.Args[i] = strings.TrimSpace(req.Args[i])
			}
		}
	}

	// Then handle all other query parameters defined in the operation
	for _, param := range op.Parameters {
		if param.In == "query" {
			value := r.URL.Query().Get(param.Name)
			// Skip "args" since we already handled it above
			if param.Name == "args" {
				continue
			}
			// Always check if the parameter exists in the query, even if empty
			if r.URL.Query().Has(param.Name) {
				req.Flags[param.Name] = value // Override body value, even if empty
			} else if param.Required {
				return nil, fmt.Errorf("required parameter %s is missing", param.Name)
			}
		}
	}

	// Validate required parameters
	for _, param := range op.Parameters {
		if param.Required {
			if param.Name == "args" && len(req.Args) == 0 {
				return nil, fmt.Errorf("required parameter %s is missing", param.Name)
			}
			if _, exists := req.Flags[param.Name]; !exists && param.Name != "args" {
				return nil, fmt.Errorf("required parameter %s is missing", param.Name)
			}
		}
	}

	return req, nil
}

// ValidateParameters validates request parameters against operation schema
func (e *CommandExecutor) ValidateParameters(req *ExecutionRequest, op *RPCOperation) error {
	for _, param := range op.Parameters {
		if param.Required {
			if param.Name == "args" {
				if len(req.Args) == 0 {
					return fmt.Errorf("required parameter %s is missing", param.Name)
				}
				continue
			}

			if _, exists := req.Flags[param.Name]; !exists {
				return fmt.Errorf("required parameter %s is missing", param.Name)
			}
		}

		// Type validation for flags
		if value, exists := req.Flags[param.Name]; exists {
			if err := e.validateParameterType(value, param.Type); err != nil {
				return fmt.Errorf("invalid type for parameter %s: %w", param.Name, err)
			}
		}
	}

	return nil
}

// validateParameterType validates a parameter value against its expected type
func (e *CommandExecutor) validateParameterType(value, paramType string) error {
	switch paramType {
	case "boolean":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("expected boolean, got %s", value)
		}
	case "integer":
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("expected integer, got %s", value)
		}
	case "number":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("expected number, got %s", value)
		}
	case "string":
		// Strings are always valid
	case "array":
		// For arrays, we expect JSON array format or comma-separated values
		// This is a simple validation - could be enhanced
		// Single value arrays are acceptable (no validation needed for simple cases)
	default:
		// Unknown types are treated as strings
	}

	return nil
}

// convertValueToString converts interface{} values to strings for flag usage
func convertValueToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		// JSON numbers are decoded as float64
		if v == float64(int64(v)) {
			// If it's a whole number, format as integer
			return fmt.Sprintf("%.0f", v)
		}
		return fmt.Sprintf("%g", v)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// extractExitCode extracts the exit code from a command execution error
func extractExitCode(err error) int {
	if err == nil {
		return 0
	}

	// Try to extract exit code from exec.ExitError
	if exitError, ok := err.(*exec.ExitError); ok {
		if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}

	// For cobra command errors, we don't have a direct exit code
	// Return 1 as a generic error code
	return 1
}

// buildCLICommand constructs the equivalent CLI command string from the operation and request
func buildCLICommand(op *RPCOperation, req *ExecutionRequest) string {
	if op == nil || op.Command == nil {
		return ""
	}

	// Start with the base command name (usually "clicky")
	baseCmd := "clicky"

	// Get the command path (e.g., "pretty", "user create")
	cmdPath := op.Command.Path()

	// Build the command parts
	parts := []string{baseCmd}

	// Add command path if it's not the root command
	if cmdPath != "" && cmdPath != "clicky" {
		parts = append(parts, cmdPath)
	}

	// Add positional arguments
	if req != nil && len(req.Args) > 0 {
		for _, arg := range req.Args {
			parts = append(parts, shellescape(arg))
		}
	}

	// Add flags
	if req != nil && len(req.Flags) > 0 {
		for flagName, flagValue := range req.Flags {
			// Check if this is a boolean flag by looking at the command's flag definition
			if op.Command.IsBoolFlag(flagName) {
				// For boolean flags
				if flagValue == "true" {
					parts = append(parts, "--"+flagName)
				}
				// Skip false boolean flags (default behavior) - don't add anything
				continue
			}

			// For non-boolean flags
			if flagValue == "" {
				// For non-boolean empty flags, include them with empty value
				parts = append(parts, "--"+flagName+"=")
			} else {
				// For flags with values, use --flag=value format
				parts = append(parts, "--"+flagName+"="+shellescape(flagValue))
			}
		}
	}

	return strings.Join(parts, " ")
}

// shellescape escapes a string for safe use in shell commands
func shellescape(s string) string {
	// If string is empty, return empty quotes
	if s == "" {
		return `""`
	}

	// If string contains no special characters, return as-is
	if !strings.ContainsAny(s, " \t\n\r\"'\\$`|&;()<>") {
		return s
	}

	// Otherwise, escape with double quotes and escape internal quotes and backslashes
	escaped := strings.ReplaceAll(s, "\\", "\\\\")      // Escape backslashes first
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"") // Escape double quotes
	return `"` + escaped + `"`
}
