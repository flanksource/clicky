package rpc

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
)

type ctxKey string

const probeKey ctxKey = "probe"

// TestExecuteCommandPrefersContextDataFunc verifies that when an operation sets
// ContextDataFunc, the executor calls it (not DataFunc) and threads req.Context
// through to it.
func TestExecuteCommandPrefersContextDataFunc(t *testing.T) {
	executor := &CommandExecutor{config: &ExecutorConfig{Enabled: true}}

	var gotCtxValue any
	dataFuncCalled := false
	op := &RPCOperation{
		Name:    "probe",
		Command: NewCobraExecutableCommand(&cobra.Command{Use: "probe"}),
		DataFunc: func(map[string]string, []string) (any, error) {
			dataFuncCalled = true
			return "from-datafunc", nil
		},
		ContextDataFunc: func(ctx context.Context, _ map[string]string, _ []string) (any, error) {
			gotCtxValue = ctx.Value(probeKey)
			return "from-context-datafunc", nil
		},
	}

	ctx := context.WithValue(context.Background(), probeKey, "carried")
	data, resp, err := executor.ExecuteCommand(op, &ExecutionRequest{Context: ctx})
	if err != nil {
		t.Fatalf("ExecuteCommand: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got %+v", resp)
	}
	if dataFuncCalled {
		t.Error("DataFunc was called despite ContextDataFunc being set")
	}
	if data != "from-context-datafunc" {
		t.Errorf("data = %v, want from-context-datafunc", data)
	}
	if gotCtxValue != "carried" {
		t.Errorf("context value = %v, want carried (context not threaded)", gotCtxValue)
	}
}

// TestExecuteCommandFallsBackToDataFunc verifies the non-context path is intact
// for operations that only set DataFunc.
func TestExecuteCommandFallsBackToDataFunc(t *testing.T) {
	executor := &CommandExecutor{config: &ExecutorConfig{Enabled: true}}
	op := &RPCOperation{
		Name:    "probe",
		Command: NewCobraExecutableCommand(&cobra.Command{Use: "probe"}),
		DataFunc: func(map[string]string, []string) (any, error) {
			return "from-datafunc", nil
		},
	}
	data, resp, err := executor.ExecuteCommand(op, &ExecutionRequest{})
	if err != nil {
		t.Fatalf("ExecuteCommand: %v", err)
	}
	if !resp.Success || data != "from-datafunc" {
		t.Errorf("data = %v success = %v, want from-datafunc/true", data, resp.Success)
	}
}

// TestExtractRequestFromHTTPCarriesContext verifies the request's context.Context
// is copied onto ExecutionRequest so a ContextDataFunc can read it.
func TestExtractRequestFromHTTPCarriesContext(t *testing.T) {
	executor := &CommandExecutor{config: &ExecutorConfig{Enabled: true}}
	op := &RPCOperation{Name: "probe", Path: "/api/v1/probe", Method: "GET"}

	r := httptest.NewRequest("GET", "/api/v1/probe", nil)
	ctx := context.WithValue(r.Context(), probeKey, "via-request")
	r = r.WithContext(ctx)

	req, err := executor.ExtractRequestFromHTTP(r, op)
	if err != nil {
		t.Fatalf("ExtractRequestFromHTTP: %v", err)
	}
	if req.Context == nil {
		t.Fatal("req.Context is nil; expected r.Context()")
	}
	if got := req.Context.Value(probeKey); got != "via-request" {
		t.Errorf("req.Context value = %v, want via-request", got)
	}
}

// TestExecutionRequestCtxDefaultsToBackground guards the nil-context fallback so
// older callers building ExecutionRequest without a context don't panic.
func TestExecutionRequestCtxDefaultsToBackground(t *testing.T) {
	req := &ExecutionRequest{}
	if req.ctx() != context.Background() {
		t.Error("ctx() should default to context.Background() when Context is nil")
	}
}
