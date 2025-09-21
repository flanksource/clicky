package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func createTestRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "testapp",
		Short: "Test application",
		Long:  "A test application for OpenAPI serve testing",
	}

	// Add a simple command
	userCmd := &cobra.Command{
		Use:   "user",
		Short: "User management",
		Long:  "Manage users in the system",
	}

	createUserCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user",
		Long:  "Create a new user with the specified details",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	var name, email string
	var active bool
	createUserCmd.Flags().StringVarP(&name, "name", "n", "", "User's name")
	createUserCmd.Flags().StringVarP(&email, "email", "e", "", "User's email")
	createUserCmd.Flags().BoolVarP(&active, "active", "a", true, "User is active")

	userCmd.AddCommand(createUserCmd)
	rootCmd.AddCommand(userCmd)

	return rootCmd
}

func TestNewSwaggerServer(t *testing.T) {
	config := &ServeConfig{
		Host:        "localhost",
		Port:        8080,
		Title:       "Test API",
		Description: "Test API Description",
		Version:     "1.0.0",
		AutoRefresh: false,
		Open:        false,
	}

	openAPIConfig := &OpenAPIConfig{
		Title:       "Test API",
		Description: "Test description",
		Version:     "1.0.0",
	}

	rootCmd := createTestRootCommand()
	server := NewSwaggerServer(config, rootCmd, openAPIConfig)

	assert.NotNil(t, server)
	assert.Equal(t, config, server.config)
	assert.Equal(t, rootCmd, server.rootCmd)
	assert.NotNil(t, server.generator)
}

func TestSwaggerServer_handleHealth(t *testing.T) {
	config := DefaultServeConfig()
	config.Version = "2.0.0"

	server := NewSwaggerServer(config, createTestRootCommand(), &OpenAPIConfig{})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var health map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &health)
	require.NoError(t, err)

	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, "2.0.0", health["version"])
	assert.Equal(t, "OpenAPI Documentation Server", health["server"])
	assert.Contains(t, health, "timestamp")
}

func TestSwaggerServer_handleOpenAPIJSON(t *testing.T) {
	config := DefaultServeConfig()
	config.Title = "Test API"
	config.Description = "Test Description"
	config.Version = "1.0.0"

	openAPIConfig := &OpenAPIConfig{
		Title:       config.Title,
		Description: config.Description,
		Version:     config.Version,
	}

	server := NewSwaggerServer(config, createTestRootCommand(), openAPIConfig)

	req := httptest.NewRequest("GET", "/api/openapi.json", nil)
	w := httptest.NewRecorder()

	server.handleOpenAPIJSON(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))

	var spec OpenAPISpec
	err := json.Unmarshal(w.Body.Bytes(), &spec)
	require.NoError(t, err)

	assert.Equal(t, "3.0.3", spec.OpenAPI)
	assert.Equal(t, "Test API", spec.Info.Title)
	assert.Equal(t, "Test Description", spec.Info.Description)
	assert.Equal(t, "1.0.0", spec.Info.Version)
	assert.NotEmpty(t, spec.Paths)
}

func TestSwaggerServer_handleOpenAPIYAML(t *testing.T) {
	config := DefaultServeConfig()
	config.Title = "Test API"

	openAPIConfig := &OpenAPIConfig{
		Title: config.Title,
	}

	server := NewSwaggerServer(config, createTestRootCommand(), openAPIConfig)

	req := httptest.NewRequest("GET", "/api/openapi.yaml", nil)
	w := httptest.NewRecorder()

	server.handleOpenAPIYAML(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/yaml", w.Header().Get("Content-Type"))
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))

	var spec OpenAPISpec
	err := yaml.Unmarshal(w.Body.Bytes(), &spec)
	require.NoError(t, err)

	assert.Equal(t, "3.0.3", spec.OpenAPI)
	assert.Equal(t, "Test API", spec.Info.Title)
}

func TestSwaggerServer_handleSwaggerUI(t *testing.T) {
	config := DefaultServeConfig()
	config.Title = "Test UI"
	config.Description = "Test Description"
	config.Version = "1.0.0"

	server := NewSwaggerServer(config, createTestRootCommand(), &OpenAPIConfig{})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	server.handleSwaggerUI(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))

	body := w.Body.String()
	assert.Contains(t, body, "Test UI")
	assert.Contains(t, body, "Test Description")
	assert.Contains(t, body, "1.0.0")
	assert.Contains(t, body, "swagger-ui-bundle.js")
	assert.Contains(t, body, "/api/openapi.json")
}

func TestSwaggerServer_handleSwaggerUI_NotFound(t *testing.T) {
	config := DefaultServeConfig()
	server := NewSwaggerServer(config, createTestRootCommand(), &OpenAPIConfig{})

	req := httptest.NewRequest("GET", "/invalid-path", nil)
	w := httptest.NewRecorder()

	server.handleSwaggerUI(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSwaggerServer_handleOpenAPIJSON_OPTIONS(t *testing.T) {
	config := DefaultServeConfig()
	server := NewSwaggerServer(config, createTestRootCommand(), &OpenAPIConfig{})

	req := httptest.NewRequest("OPTIONS", "/api/openapi.json", nil)
	w := httptest.NewRecorder()

	server.handleOpenAPIJSON(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type", w.Header().Get("Access-Control-Allow-Headers"))
	assert.Empty(t, w.Body.String())
}

func TestDefaultServeConfig(t *testing.T) {
	config := DefaultServeConfig()

	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 8080, config.Port)
	assert.Equal(t, "CLI API", config.Title)
	assert.Equal(t, "Generated API documentation from CLI commands", config.Description)
	assert.Equal(t, "1.0.0", config.Version)
	assert.False(t, config.AutoRefresh)
	assert.False(t, config.Open)
}

func TestSwaggerServer_StartAndShutdown(t *testing.T) {
	t.Skip("Skipping integration test that requires network resources")

	config := DefaultServeConfig()
	config.Port = 0 // Use random available port

	server := NewSwaggerServer(config, createTestRootCommand(), &OpenAPIConfig{})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Start server in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// Wait for server to shut down
	select {
	case err := <-errChan:
		assert.Contains(t, err.Error(), "context canceled")
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not shut down within timeout")
	}
}

func TestTemplateData(t *testing.T) {
	data := TemplateData{
		Title:       "Test Title",
		Description: "Test Description",
		Version:     "2.0.0",
		Timestamp:   "2023-01-01 12:00:00 UTC",
		AutoRefresh: true,
	}

	assert.Equal(t, "Test Title", data.Title)
	assert.Equal(t, "Test Description", data.Description)
	assert.Equal(t, "2.0.0", data.Version)
	assert.Equal(t, "2023-01-01 12:00:00 UTC", data.Timestamp)
	assert.True(t, data.AutoRefresh)
}

// Integration test that verifies the complete flow
func TestSwaggerServer_Integration(t *testing.T) {
	// Create test server
	config := DefaultServeConfig()
	config.Title = "Integration Test API"
	config.Description = "Test API for integration testing"
	config.Version = "1.5.0"

	openAPIConfig := &OpenAPIConfig{
		Title:       config.Title,
		Description: config.Description,
		Version:     config.Version,
	}

	rootCmd := createTestRootCommand()
	server := NewSwaggerServer(config, rootCmd, openAPIConfig)

	// Test all endpoints
	endpoints := []struct {
		path     string
		method   string
		contains []string
	}{
		{
			path:   "/",
			method: "GET",
			contains: []string{
				"Integration Test API",
				"Test API for integration testing",
				"1.5.0",
				"swagger-ui-bundle.js",
			},
		},
		{
			path:   "/api/openapi.json",
			method: "GET",
			contains: []string{
				`"openapi": "3.0.3"`,
				`"title": "Integration Test API"`,
				`"version": "1.5.0"`,
			},
		},
		{
			path:   "/api/openapi.yaml",
			method: "GET",
			contains: []string{
				"openapi: 3.0.3",
				"title: Integration Test API",
				"version: 1.5.0",
			},
		},
		{
			path:   "/health",
			method: "GET",
			contains: []string{
				`"status":"healthy"`,
				`"version":"1.5.0"`,
				`"server":"OpenAPI Documentation Server"`,
			},
		},
	}

	for _, endpoint := range endpoints {
		t.Run(fmt.Sprintf("%s %s", endpoint.method, endpoint.path), func(t *testing.T) {
			req := httptest.NewRequest(endpoint.method, endpoint.path, nil)
			w := httptest.NewRecorder()

			switch endpoint.path {
			case "/":
				server.handleSwaggerUI(w, req)
			case "/api/openapi.json":
				server.handleOpenAPIJSON(w, req)
			case "/api/openapi.yaml":
				server.handleOpenAPIYAML(w, req)
			case "/health":
				server.handleHealth(w, req)
			}

			assert.Equal(t, http.StatusOK, w.Code)

			body := w.Body.String()
			for _, content := range endpoint.contains {
				assert.Contains(t, body, content, "Response should contain: %s", content)
			}
		})
	}
}
