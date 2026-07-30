package rpc

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleExecuteCommand_DataFuncReturnsArray is a regression test for the bug
// where DataFunc-backed entity list/get operations returned the empty
// ExecutionResponse envelope ({success, exit_code, cli}) instead of their
// structured payload when the client asked for JSON. The handler substituted
// `metadata` for `data` on every structured wire format; DataFunc ops carry
// their payload in `data` (metadata.Output is empty), so the result was dropped
// and the web UI showed no rows. The fix gates the substitution on
// !metadata.DataIsStructured.
func TestHandleExecuteCommand_DataFuncReturnsArray(t *testing.T) {
	type row struct {
		Name string `json:"name"`
	}
	want := []row{{Name: "dev"}, {Name: "prod"}}

	op := RPCOperation{
		Name:   "config list",
		Path:   "/api/v1/config",
		Method: "GET",
		DataFunc: func(_ map[string]string, _ []string) (any, error) {
			return want, nil
		},
	}
	service := &RPCService{Name: "api", Operations: []RPCOperation{op}}
	executor := NewCommandExecutor(service, &ExecutorConfig{Enabled: true, SkipPreRun: true, PathPrefix: "/api/v1"})

	server := &SwaggerServer{
		config:   &ServeConfig{Executor: &ExecutorConfig{Enabled: true}},
		executor: executor,
	}

	req := httptest.NewRequest("GET", "/api/v1/config", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	server.handleExecuteCommand(w, req)

	res := w.Result()
	require.Equal(t, 200, res.StatusCode)

	var got []row
	require.NoError(t, json.NewDecoder(res.Body).Decode(&got),
		"JSON body must be the data array, not the {success,exit_code,cli} envelope")
	assert.Equal(t, want, got)
}

func TestHandleExecuteCommand_DataFuncPreservesStructuredDataOnError(t *testing.T) {
	type failedRun struct {
		ActivityGUID string `json:"activityGuid"`
		Exception    string `json:"exception"`
	}
	want := failedRun{
		ActivityGUID: "8C055EB2-6F5C-4E46-945F-901AC8504865",
		Exception:    "Exception in doMath()",
	}
	op := RPCOperation{
		Name:   "activity execute",
		Path:   "/api/v1/activity/{id}/execute",
		Method: "POST",
		DataFunc: func(_ map[string]string, _ []string) (any, error) {
			return want, errors.New("activity processing failed")
		},
	}
	service := &RPCService{Name: "api", Operations: []RPCOperation{op}}
	executor := NewCommandExecutor(service, &ExecutorConfig{Enabled: true, SkipPreRun: true, PathPrefix: "/api/v1"})
	server := &SwaggerServer{
		config:   &ServeConfig{Executor: &ExecutorConfig{Enabled: true}},
		executor: executor,
	}

	req := httptest.NewRequest("POST", "/api/v1/activity/8C055EB2-6F5C-4E46-945F-901AC8504865/execute", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	server.handleExecuteCommand(w, req)

	res := w.Result()
	require.Equal(t, 500, res.StatusCode)
	assert.Equal(t, "false", res.Header.Get("X-Execution-Success"))
	assert.Equal(t, "activity processing failed", res.Header.Get("X-Error"))
	var got failedRun
	require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
	assert.Equal(t, want, got)
}
