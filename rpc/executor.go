package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/flanksource/commons/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// CommandExecutor handles dynamic execution of Cobra commands via HTTP requests
type CommandExecutor struct {
	service    *RPCService              // Pre-converted command tree
	operations map[string]*RPCOperation // path+method -> operation lookup
	config     *ExecutorConfig          // Execution configuration
	mutex      sync.Mutex               // Protects global stdout/stderr replacement
}

// ExecutionRequest represents parameters for command execution
type ExecutionRequest struct {
	Args  []string          `json:"args,omitempty"`  // Positional arguments
	Flags map[string]string `json:"flags,omitempty"` // Flag values
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

// FindOperation finds an RPC operation by HTTP method and path
func (e *CommandExecutor) FindOperation(method, path string) *RPCOperation {
	key := strings.ToUpper(method) + ":" + path
	return e.operations[key]
}

// ExecuteCommand executes a Cobra command with the given parameters
// Returns: (parsedData, metadata, error)
// - parsedData: The actual command output (parsed if possible, or ExecutionResponse wrapper)
// - metadata: Execution metadata for HTTP headers (CLI command, exit code, success)
// - error: Execution error if any
func (e *CommandExecutor) ExecuteCommand(op *RPCOperation, req *ExecutionRequest) (any, *ExecutionResponse, error) {
	if !e.config.Enabled {
		resp := &ExecutionResponse{
			Success: false,
			Error:   "Command execution is disabled",
			Input:   req,
			CLI:     buildCLICommand(op, req),
		}
		return resp, resp, fmt.Errorf("command execution is disabled")
	}

	cmd := op.Command
	if cmd == nil {
		resp := &ExecutionResponse{
			Success: false,
			Error:   "No command associated with operation",
			Input:   req,
			CLI:     buildCLICommand(op, req),
		}
		return resp, resp, fmt.Errorf("no command found for operation %s", op.Name)
	}

	// Store original hooks if we need to skip pre-runs
	var (
		origPreRun            func(*cobra.Command, []string)
		origPreRunE           func(*cobra.Command, []string) error
		origPersistentPreRun  func(*cobra.Command, []string)
		origPersistentPreRunE func(*cobra.Command, []string) error
	)

	if e.config.SkipPreRun {
		origPreRun = cmd.PreRun
		origPreRunE = cmd.PreRunE
		origPersistentPreRun = cmd.PersistentPreRun
		origPersistentPreRunE = cmd.PersistentPreRunE

		// Clear pre-run hooks
		cmd.PreRun = nil
		cmd.PreRunE = nil
		cmd.PersistentPreRun = nil
		cmd.PersistentPreRunE = nil
	}

	// Restore hooks after execution
	defer func() {
		if e.config.SkipPreRun {
			cmd.PreRun = origPreRun
			cmd.PreRunE = origPreRunE
			cmd.PersistentPreRun = origPersistentPreRun
			cmd.PersistentPreRunE = origPersistentPreRunE
		}
	}()

	// Set flags from request
	if req.Flags != nil {
		for flagName, flagValue := range req.Flags {
			if flag := cmd.Flags().Lookup(flagName); flag != nil {
				if err := flag.Value.Set(flagValue); err != nil {
					resp := &ExecutionResponse{
						Success: false,
						Error:   fmt.Sprintf("Invalid value for flag %s: %v", flagName, err),
						Input:   req,
						CLI:     buildCLICommand(op, req),
					}
					return resp, resp, err
				}
				// IMPORTANT: Mark the flag as changed so required flag validation passes
				flag.Changed = true
			}
		}
	}

	// Create a new command instance to avoid modifying the original
	execCmd := &cobra.Command{
		Use:   cmd.Use,
		Short: cmd.Short,
		Long:  cmd.Long,
		Run:   cmd.Run,
		RunE:  cmd.RunE,
		Args:  cmd.Args,
	}

	// Copy flags to the execution command
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		execCmd.Flags().AddFlag(flag)
	})

	// Set arguments
	args := []string{}
	if req.Args != nil {
		args = req.Args
	}

	// Execute with global output capture
	stdoutStr, stderrStr, err := e.executeWithGlobalCapture(execCmd, args)

	// Extract exit code
	exitCode := extractExitCode(err)

	// Parse the output to return actual data instead of wrapper
	parsedData, parseErr := parseCommandOutput(stdoutStr, stderrStr, req)

	// Build response with metadata for headers
	response := &ExecutionResponse{
		Success:  err == nil,
		Stdout:   stdoutStr,
		Stderr:   stderrStr,
		Output:   stdoutStr + stderrStr,
		ExitCode: exitCode,
		CLI:      buildCLICommand(op, req),
	}

	if err != nil {
		response.Error = err.Error()
		response.Message = "Command execution failed"
		response.Input = req
		// Return error response for failed commands
		return response, response, err
	}

	// If parse succeeded, return the parsed data directly with metadata
	if parseErr == nil && parsedData != nil {
		return parsedData, response, nil
	}

	// Fallback to response wrapper if parsing failed
	response.Message = "Command executed successfully"
	return response, response, nil
}

// parseCommandOutput attempts to parse command output into structured data
func parseCommandOutput(stdout, stderr string, req *ExecutionRequest) (any, error) {
	// If there's no stdout, nothing to parse
	if stdout == "" {
		return nil, fmt.Errorf("no output to parse")
	}

	// Try to detect format from output content
	trimmed := strings.TrimSpace(stdout)

	// Try JSON first (most common for CLI tools)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var data any
		if err := json.Unmarshal([]byte(trimmed), &data); err == nil {
			return data, nil
		}
	}

	// If parsing fails, return nil to fallback to wrapper
	return nil, fmt.Errorf("unable to parse output")
}

// executeWithGlobalCapture executes a command while capturing ALL output including direct os.Stdout/os.Stderr writes
func (e *CommandExecutor) executeWithGlobalCapture(execCmd *cobra.Command, args []string) (stdout, stderr string, err error) {
	// Use mutex to ensure thread safety when replacing global file descriptors
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// Create capture buffers
	var stdoutBuf, stderrBuf bytes.Buffer

	// Store original file descriptors
	originalStdout := os.Stdout
	originalStderr := os.Stderr
	originalArgs := os.Args

	// Ensure restoration even on panic
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
		os.Args = originalArgs
	}()

	// Create pipes for capturing output
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return "", "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	defer func() {
		if err := stdoutReader.Close(); err != nil {
			logger.Errorf("failed to close stdout reader: %v", err)
		}
	}()

	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutWriter.Close()
		return "", "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	defer func() {
		if err := stderrReader.Close(); err != nil {
			logger.Errorf("failed to close stderr reader: %v", err)
		}
	}()

	// Replace global stdout/stderr
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	// Also set cobra command outputs to use the same writers
	execCmd.SetOut(stdoutWriter)
	execCmd.SetErr(stderrWriter)

	// Start goroutines to copy from pipes to buffers
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if _, err := io.Copy(&stdoutBuf, stdoutReader); err != nil {
			// Log error but continue
			fmt.Printf("Warning: failed to copy stdout: %v\n", err)
		}
	}()

	go func() {
		defer wg.Done()
		if _, err := io.Copy(&stderrBuf, stderrReader); err != nil {
			// Log error but continue
			fmt.Printf("Warning: failed to copy stderr: %v\n", err)
		}
	}()

	// Execute the command
	execCmd.SetArgs(args)
	cmdErr := execCmd.Execute()

	// Close writers to signal end of output and flush remaining data
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()

	// Wait for all output to be copied
	wg.Wait()

	return stdoutBuf.String(), stderrBuf.String(), cmdErr
}

// ExtractRequestFromHTTP extracts execution parameters from HTTP request
func (e *CommandExecutor) ExtractRequestFromHTTP(r *http.Request, op *RPCOperation) (*ExecutionRequest, error) {
	req := &ExecutionRequest{
		Flags: make(map[string]string),
	}

	// Extract JSON body for POST/PUT requests FIRST (so query params can override)
	if r.Method == "POST" || r.Method == "PUT" {
		if r.Header.Get("Content-Type") == "application/json" {
			var bodyData map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&bodyData); err != nil {
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
	cmdPath := getCommandPath(op.Command)

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
			if flag := op.Command.Flags().Lookup(flagName); flag != nil && flag.Value.Type() == "bool" {
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
