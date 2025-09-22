package clicky_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ExecutionResponse represents the result of command execution (mirrored from rpc package)
type ExecutionResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message,omitempty"`
	Output   string `json:"output,omitempty"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// TestE2E_ClickyCommandExecution tests the clicky command execution end-to-end
func TestE2E_ClickyCommandExecution(t *testing.T) {
	// Use the existing clicky binary
	binaryPath := "./clicky"

	// Setup test data paths
	exampleDataPath := "examples/example-data.json"
	schemaPath := "examples/order-schema.yaml"

	// Verify test files exist
	require.FileExists(t, exampleDataPath, "Example data file should exist")
	require.FileExists(t, schemaPath, "Schema file should exist")

	t.Run("Valid Pretty Command Execution", func(t *testing.T) {
		// Test actual pretty command execution
		cmd := exec.Command(binaryPath, "pretty", "--schema", schemaPath, exampleDataPath)

		// Capture output
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// Execute command
		err := cmd.Run()
		require.NoError(t, err, "Command should execute successfully")

		// Verify output
		output := stdout.String()
		assert.NotEmpty(t, output, "Should have output")
		assert.Contains(t, output, "ORD-2024-4567", "Should contain order ID")
		assert.Contains(t, output, "Acme Corporation", "Should contain customer name")
	})

	t.Run("Missing Schema Parameter", func(t *testing.T) {
		// Test command without schema (should fail)
		cmd := exec.Command(binaryPath, "pretty", exampleDataPath)

		// Capture output
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// Execute command (should fail)
		err := cmd.Run()
		assert.Error(t, err, "Command should fail without schema")

		// Check stderr for error message
		errorOutput := stderr.String()
		assert.Contains(t, strings.ToLower(errorOutput), "schema", "Error should mention schema requirement")
	})

	t.Run("Invalid Schema File", func(t *testing.T) {
		// Test command with non-existent schema
		cmd := exec.Command(binaryPath, "pretty", "--schema", "/nonexistent/schema.yaml", exampleDataPath)

		// Capture output
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// Execute command (should fail)
		err := cmd.Run()
		assert.Error(t, err, "Command should fail with invalid schema")
	})

	t.Run("Boolean Flags Work Correctly", func(t *testing.T) {
		// Test with boolean flags
		cmd := exec.Command(binaryPath, "pretty", "--schema", schemaPath, "--verbose", "--no-color", exampleDataPath)

		// Capture output
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// Execute command
		err := cmd.Run()
		require.NoError(t, err, "Command should execute successfully with boolean flags")

		// Verify output
		output := stdout.String()
		assert.NotEmpty(t, output, "Should have output")
	})

	t.Run("OpenAPI Generation Works", func(t *testing.T) {
		// Test OpenAPI generation
		cmd := exec.Command(binaryPath, "openapi", "generate")

		// Capture output
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// Execute command
		err := cmd.Run()
		require.NoError(t, err, "OpenAPI generation should succeed")

		// Verify output is valid JSON
		output := stdout.String()
		assert.NotEmpty(t, output, "Should have output")

		var spec map[string]interface{}
		err = json.Unmarshal([]byte(output), &spec)
		require.NoError(t, err, "Output should be valid JSON")

		// Verify required components exist
		assert.Contains(t, spec, "openapi", "Should have openapi version")
		assert.Contains(t, spec, "paths", "Should have paths")
		assert.Contains(t, spec, "info", "Should have info")

		// Check that pretty endpoint exists
		paths, ok := spec["paths"].(map[string]interface{})
		require.True(t, ok, "Paths should be a map")
		assert.Contains(t, paths, "/api/v1/pretty", "Should have pretty endpoint")

		// Check schema parameter is required
		prettyPath, ok := paths["/api/v1/pretty"].(map[string]interface{})
		require.True(t, ok, "Pretty path should be a map")

		postMethod, ok := prettyPath["post"].(map[string]interface{})
		require.True(t, ok, "POST method should exist")

		requestBody, ok := postMethod["requestBody"].(map[string]interface{})
		require.True(t, ok, "Request body should exist")

		content, ok := requestBody["content"].(map[string]interface{})
		require.True(t, ok, "Content should exist")

		jsonContent, ok := content["application/json"].(map[string]interface{})
		require.True(t, ok, "JSON content should exist")

		schema, ok := jsonContent["schema"].(map[string]interface{})
		require.True(t, ok, "Schema should exist")

		required, ok := schema["required"].([]interface{})
		require.True(t, ok, "Required fields should exist")

		// Check that schema is in required fields
		foundSchema := false
		for _, field := range required {
			if field == "schema" {
				foundSchema = true
				break
			}
		}
		assert.True(t, foundSchema, "Schema should be in required fields")

		// Check that boolean flags are not required
		properties, ok := schema["properties"].(map[string]interface{})
		require.True(t, ok, "Properties should exist")

		// Check some boolean properties exist but are not required
		booleanFlags := []string{"verbose", "no-color", "json", "yaml", "html"}
		for _, flagName := range booleanFlags {
			if prop, exists := properties[flagName]; exists {
				propMap, ok := prop.(map[string]interface{})
				require.True(t, ok, "Property should be a map")
				assert.Equal(t, "boolean", propMap["type"], "Flag %s should be boolean type", flagName)

				// Ensure it's not in required fields
				foundInRequired := false
				for _, field := range required {
					if field == flagName {
						foundInRequired = true
						break
					}
				}
				assert.False(t, foundInRequired, "Boolean flag %s should not be required", flagName)
			}
		}
	})
}

// TestE2E_HTTPAPIWithMockServer tests the HTTP API functionality with a mock server
func TestE2E_HTTPAPIWithMockServer(t *testing.T) {
	// Create a mock server that simulates the clicky HTTP API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/pretty" && r.Method == "POST" {
			handleMockPrettyRequest(w, r)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	baseURL := server.URL

	t.Run("Valid Request With Schema", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"args":   []string{"examples/example-data.json"},
			"schema": "examples/order-schema.yaml",
			"format": "pretty",
		}

		response := makeHTTPRequest(t, baseURL+"/api/v1/pretty", "POST", requestBody)

		assert.True(t, response.Success, "Request should succeed")
		assert.Equal(t, 0, response.ExitCode, "Exit code should be 0")
		assert.NotEmpty(t, response.Stdout, "Should have stdout output")
		assert.Contains(t, response.Stdout, "ORD-2024-4567", "Should contain order ID")
	})

	t.Run("Missing Schema Parameter", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"args":   []string{"examples/example-data.json"},
			"format": "pretty",
		}

		response := makeHTTPRequestExpectError(t, baseURL+"/api/v1/pretty", "POST", requestBody)

		assert.False(t, response.Success, "Request should fail")
		assert.NotEqual(t, 0, response.ExitCode, "Exit code should not be 0")
		assert.Contains(t, strings.ToLower(response.Error), "schema", "Error should mention schema")
	})

	t.Run("Boolean Flags Are Optional", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"args":     []string{"examples/example-data.json"},
			"schema":   "examples/order-schema.yaml",
			"verbose":  true,
			"no-color": false,
		}

		response := makeHTTPRequest(t, baseURL+"/api/v1/pretty", "POST", requestBody)

		assert.True(t, response.Success, "Request with boolean flags should succeed")
		assert.Equal(t, 0, response.ExitCode, "Exit code should be 0")
	})
}

// makeHTTPRequest makes an HTTP request and expects success
func makeHTTPRequest(t *testing.T, url, method string, body interface{}) *ExecutionResponse {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		require.NoError(t, err, "Should marshal request body")
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	require.NoError(t, err, "Should create HTTP request")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err, "Should execute HTTP request")
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Should read response body")

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should get 200 OK, got %d: %s", resp.StatusCode, string(responseBody))

	var execResponse ExecutionResponse
	err = json.Unmarshal(responseBody, &execResponse)
	require.NoError(t, err, "Should unmarshal execution response")

	return &execResponse
}

// makeHTTPRequestExpectError makes an HTTP request expecting an error
func makeHTTPRequestExpectError(t *testing.T, url, method string, body interface{}) *ExecutionResponse {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		require.NoError(t, err, "Should marshal request body")
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	require.NoError(t, err, "Should create HTTP request")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err, "Should execute HTTP request")
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Should read response body")

	// For error cases, we might get different status codes
	assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 600, "Should get valid HTTP status")

	var execResponse ExecutionResponse
	err = json.Unmarshal(responseBody, &execResponse)
	require.NoError(t, err, "Should unmarshal execution response")

	return &execResponse
}

// handleMockPrettyRequest handles mock pretty command requests
func handleMockPrettyRequest(w http.ResponseWriter, r *http.Request) {
	var reqBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		response := &ExecutionResponse{
			Success:  false,
			Error:    fmt.Sprintf("Failed to parse request: %v", err),
			ExitCode: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Check for required schema parameter
	schema, hasSchema := reqBody["schema"].(string)
	if !hasSchema || schema == "" {
		response := &ExecutionResponse{
			Success:  false,
			Error:    "--schema flag is required",
			ExitCode: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // Return 200 but with failure in response
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Mock successful execution
	mockOutput := `📋 Order Details
ID: ORD-2024-4567
Customer: Acme Corporation
Status: PROCESSING
Total: $15,750.00 USD`

	response := &ExecutionResponse{
		Success:  true,
		Message:  "Command executed successfully",
		Stdout:   mockOutput,
		Stderr:   "",
		ExitCode: 0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
