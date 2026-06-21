package rpc

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRequestFromContextRoundTrip verifies the originating *http.Request stashed
// by ContextWithRequest is retrievable, and absent on a bare context.
func TestRequestFromContextRoundTrip(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/v1/connection", nil)
	ctx := ContextWithRequest(context.Background(), r)

	got, ok := RequestFromContext(ctx)
	if !ok || got != r {
		t.Fatalf("RequestFromContext = (%v, %v), want the stashed request", got, ok)
	}
	if _, ok := RequestFromContext(context.Background()); ok {
		t.Error("RequestFromContext on a bare context should report absent")
	}
}

// TestExtractRequestFromHTTPPreservesNestedBody verifies that after the executor
// flattens the body to string flags, a context-based handler can still read the
// full nested JSON from the wrapped request's (re-buffered) body.
func TestExtractRequestFromHTTPPreservesNestedBody(t *testing.T) {
	executor := &CommandExecutor{config: &ExecutorConfig{Enabled: true}}
	op := &RPCOperation{Name: "connection", Path: "/api/v1/connection", Method: "POST"}

	const payload = `{"name":"pg","type":"postgres","properties":{"sslmode":"disable"}}`
	r := httptest.NewRequest("POST", "/api/v1/connection", strings.NewReader(payload))
	r.Header.Set("Content-Type", "application/json")

	req, err := executor.ExtractRequestFromHTTP(r, op)
	if err != nil {
		t.Fatalf("ExtractRequestFromHTTP: %v", err)
	}

	// Top-level scalar still flattened for the legacy flag path.
	if req.Flags["name"] != "pg" {
		t.Errorf("flag name = %q, want pg", req.Flags["name"])
	}

	// A context handler reads the raw nested body off the wrapped request.
	wrapped, ok := RequestFromContext(req.Context)
	if !ok {
		t.Fatal("request not found in context")
	}
	raw, err := io.ReadAll(wrapped.Body)
	if err != nil {
		t.Fatalf("read wrapped body: %v", err)
	}
	var decoded struct {
		Name       string            `json:"name"`
		Type       string            `json:"type"`
		Properties map[string]string `json:"properties"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal wrapped body: %v", err)
	}
	if decoded.Name != "pg" || decoded.Type != "postgres" || decoded.Properties["sslmode"] != "disable" {
		t.Errorf("nested body lost: %+v", decoded)
	}
}
