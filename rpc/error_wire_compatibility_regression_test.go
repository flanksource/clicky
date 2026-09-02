package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	compatibilityBackendError = "private backend detail"
	compatibilityStderr       = "retry later"
	compatibilityCommandError = "operation exploded"
)

func TestRPCDefaultErrorsPreserveLegacyPlainTextResponses(t *testing.T) {
	lookupOperation := RPCOperation{
		Name:   "widget lookup",
		Method: http.MethodGet,
		Path:   "/api/v1/widget",
		Parameters: []RPCParameter{{
			Name: "tenant", In: "query", Type: "string", Required: true,
		}},
		LookupFunc: func(map[string]string, []string) (any, error) {
			return nil, errors.New(compatibilityBackendError)
		},
	}
	plainOperation := RPCOperation{
		Name: "plain widget list", Method: http.MethodGet, Path: "/api/v1/plain-widget",
	}
	server := newWireCompatibilityServer(t, nil, lookupOperation, plainOperation)

	tests := []struct {
		name        string
		method      string
		target      string
		wantStatus  int
		wantBody    string
		contentType string
	}{
		{
			name: "unknown operation", method: http.MethodGet, target: "/api/v1/missing",
			wantStatus: http.StatusNotFound, wantBody: "No operation found for GET /api/v1/missing\n",
			contentType: "text/plain; charset=utf-8",
		},
		{
			name: "missing lookup", method: http.MethodGet, target: "/api/v1/plain-widget?__lookup=filters",
			wantStatus: http.StatusNotFound, wantBody: "No lookup found for GET /api/v1/plain-widget\n",
			contentType: "text/plain; charset=utf-8",
		},
		{
			name: "lookup extraction failure", method: http.MethodGet, target: "/api/v1/widget?__lookup=filters",
			wantStatus:  http.StatusBadRequest,
			wantBody:    "Failed to extract parameters: required parameter tenant is missing\n",
			contentType: "text/plain; charset=utf-8",
		},
		{
			name: "lookup backend failure", method: http.MethodGet, target: "/api/v1/widget?tenant=acme&__lookup=filters",
			wantStatus: http.StatusBadRequest, wantBody: compatibilityBackendError + "\n",
			contentType: "text/plain; charset=utf-8",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.target, nil)

			server.handleExecuteCommand(response, request)

			require.Equal(t, test.wantStatus, response.Code)
			assert.Equal(t, test.contentType, response.Header().Get("Content-Type"))
			assert.Equal(t, test.wantBody, response.Body.String())
			assert.Empty(t, response.Header().Get("X-Trace-ID"))
		})
	}
}

func TestRPCDefaultValidationFailurePreservesExecutionResponse(t *testing.T) {
	operation := RPCOperation{
		Name: "retry widget", Method: http.MethodPost, Path: "/api/v1/widget/retry",
		Parameters: []RPCParameter{{
			Name: "retries", In: "query", Type: "integer",
		}},
		DataFunc: func(map[string]string, []string) (any, error) {
			return map[string]any{"unexpected": true}, nil
		},
	}
	server := newWireCompatibilityServer(t, nil, operation)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/widget/retry?retries=nope", nil)
	request.Header.Set("Accept", "application/json")

	server.handleExecuteCommand(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "application/json", response.Header().Get("Content-Type"))
	assert.Equal(t, "false", response.Header().Get("X-Execution-Success"))
	assert.Equal(t, "Parameter validation failed: invalid type for parameter retries: expected integer, got nope", response.Header().Get("X-Error"))
	assert.Empty(t, response.Header().Get("X-Stderr"))
	assert.Empty(t, response.Header().Get("X-Trace-ID"))

	var body ExecutionResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.False(t, body.Success)
	assert.Equal(t, "Parameter validation failed: invalid type for parameter retries: expected integer, got nope", body.Error)
	require.NotNil(t, body.Input)
	assert.Equal(t, map[string]string{"retries": "nope"}, body.Input.Flags)
}

func TestRPCDefaultCommandFailurePreservesExecutionResponseAndDiagnosticHeaders(t *testing.T) {
	command := &cobra.Command{
		Use: "explode",
		RunE: func(command *cobra.Command, _ []string) error {
			_, _ = fmt.Fprint(command.ErrOrStderr(), compatibilityStderr)
			return errors.New(compatibilityCommandError)
		},
	}
	operation := RPCOperation{
		Name: "explode widget", Method: http.MethodPost, Path: "/api/v1/widget/explode",
		Command: NewCobraExecutableCommand(command),
	}
	server := newWireCompatibilityServer(t, nil, operation)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/widget/explode",
		bytes.NewBufferString(`{"args":["widget-17"]}`),
	)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")

	server.handleExecuteCommand(response, request)

	require.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Equal(t, "application/json", response.Header().Get("Content-Type"))
	assert.Equal(t, "false", response.Header().Get("X-Execution-Success"))
	assert.Equal(t, compatibilityCommandError, response.Header().Get("X-Error"))
	assert.Equal(t, compatibilityStderr, response.Header().Get("X-Stderr"))
	assert.Empty(t, response.Header().Get("X-Trace-ID"))

	var body ExecutionResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.False(t, body.Success)
	assert.Equal(t, compatibilityCommandError, body.Error)
	assert.Equal(t, compatibilityStderr, body.Stderr)
	require.NotNil(t, body.Input)
	assert.Equal(t, []string{"widget-17"}, body.Input.Args)
}

func TestRPCStructuredErrorsRequireExplicitOptIn(t *testing.T) {
	server := newWireCompatibilityServer(t, func(config *ServeConfig) {
		config.StructuredErrorResponses = true
	})
	response := httptest.NewRecorder()

	server.handleExecuteCommand(response, httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil))

	require.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "application/json", response.Header().Get("Content-Type"))
	var body map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, "operation_not_found", body["code"])
	assert.Equal(t, "No operation found for GET /api/v1/missing", body["message"])
	trace, ok := body["trace"].(string)
	require.True(t, ok)
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{32}$`), trace)
	assert.Equal(t, trace, response.Header().Get("X-Trace-ID"))
}

func TestRPCStructuredErrorDetailsRemainVisibleUnlessExplicitlyHidden(t *testing.T) {
	operation := RPCOperation{
		Name: "fail widget", Method: http.MethodPost, Path: "/api/v1/widget/fail",
		DataFunc: func(map[string]string, []string) (any, error) {
			return nil, errors.New(compatibilityBackendError)
		},
	}
	tests := []struct {
		name        string
		hideDetails bool
		wantMessage string
	}{
		{name: "visible by default", wantMessage: compatibilityBackendError},
		{name: "hidden when configured", hideDetails: true, wantMessage: "internal server error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newWireCompatibilityServer(t, func(config *ServeConfig) {
				config.StructuredErrorResponses = true
				config.HideErrorDetails = test.hideDetails
			}, operation)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/widget/fail", nil)

			server.handleExecuteCommand(response, request)

			require.Equal(t, http.StatusInternalServerError, response.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			assert.Equal(t, "internal_error", body["code"])
			assert.Equal(t, test.wantMessage, body["message"])
			assert.NotEmpty(t, body["trace"])
			assert.Equal(t, body["trace"], response.Header().Get("X-Trace-ID"))
			if test.hideDetails {
				assert.NotContains(t, response.Body.String(), compatibilityBackendError)
			}
		})
	}
}

func newWireCompatibilityServer(t *testing.T, configure func(*ServeConfig), operations ...RPCOperation) *SwaggerServer {
	t.Helper()
	config := &ServeConfig{
		Title: "Compatibility API", Version: "1.0.0", SkipHealth: true,
		Executor: &ExecutorConfig{Enabled: true, SkipPreRun: true, PathPrefix: "/api/v1"},
	}
	if configure != nil {
		configure(config)
	}
	server := NewSwaggerServer(config, &cobra.Command{Use: "compatibility-test"}, &OpenAPIConfig{})
	server.executor = NewCommandExecutor(
		&RPCService{Name: "compatibility", Operations: operations},
		config.Executor,
	)
	return server
}
