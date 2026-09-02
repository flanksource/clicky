package rpc

import (
	"maps"
	"slices"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSwaggerServerPreservesDefaultOpenAPIConfigWhenNil(t *testing.T) {
	server := NewSwaggerServer(&ServeConfig{}, &cobra.Command{Use: "test"}, nil)

	require.NotNil(t, server.generator.config)
	assert.Equal(t, "API", server.generator.config.Title)
	assert.Equal(t, "Generated API from CLI commands", server.generator.config.Description)
	assert.Equal(t, "1.0.0", server.generator.config.Version)
}

func TestOpenAPIGenerator_DefaultErrorResponsesRemainLegacyCompatible(t *testing.T) {
	operation := openAPITestOperation(t, OpenAPIConfig{})

	require.Equal(t, []string{"200", "400", "500"}, slices.Sorted(maps.Keys(operation.Responses)))
	for status, description := range map[string]string{
		"400": "Bad Request",
		"500": "Internal Server Error",
	} {
		response := operation.Responses[status]
		assert.Equal(t, description, response.Description)
		assert.Empty(t, response.Headers)

		schema := requireOpenAPIResponseSchema(t, response)
		assert.Equal(t, []string{"cli", "error", "exit_code", "input", "message", "output", "stderr", "stdout", "success"}, slices.Sorted(maps.Keys(schema.Properties)))
		assert.ElementsMatch(t, []string{"success", "exit_code"}, schema.Required)
	}
}

func TestOpenAPIGenerator_StructuredErrorResponsesAreExplicitlyOptedIn(t *testing.T) {
	operation := openAPITestOperation(t, OpenAPIConfig{StructuredErrorResponses: true})

	require.Equal(t, []string{"200", "400", "404", "405", "406", "500"}, slices.Sorted(maps.Keys(operation.Responses)))
	for status, description := range map[string]string{
		"400": "Bad Request",
		"404": "Not Found",
		"405": "Method Not Allowed",
		"406": "Not Acceptable",
		"500": "Internal Server Error",
	} {
		response := operation.Responses[status]
		assert.Equal(t, description, response.Description)
		traceHeader, ok := response.Headers["X-Trace-ID"]
		require.True(t, ok)
		assert.True(t, traceHeader.Required)
		require.NotNil(t, traceHeader.Schema)
		assert.Equal(t, "string", traceHeader.Schema.Type)

		schema := requireOpenAPIResponseSchema(t, response)
		assert.ElementsMatch(t, []string{"code", "message", "trace"}, schema.Required)
		for _, field := range []string{"code", "message", "trace", "hint", "context", "details", "stacktrace", "truncated"} {
			assert.Contains(t, schema.Properties, field)
		}
		assert.NotContains(t, schema.Properties, "success")
	}
}

func openAPITestOperation(t *testing.T, config OpenAPIConfig) OpenAPIOperation {
	t.Helper()
	spec := NewOpenAPIGenerator(&config).GenerateFromService(&RPCService{
		Operations: []RPCOperation{{
			Name:   "widgets get",
			Method: "GET",
			Path:   "/api/v1/widgets/{id}",
		}},
	})
	operation, ok := spec.Paths["/api/v1/widgets/{id}"]["get"]
	require.True(t, ok)
	return operation
}

func requireOpenAPIResponseSchema(t *testing.T, response OpenAPIResponse) *OpenAPISchema {
	t.Helper()
	mediaType, ok := response.Content["application/json"]
	require.True(t, ok)
	require.NotNil(t, mediaType.Schema)
	return mediaType.Schema
}
