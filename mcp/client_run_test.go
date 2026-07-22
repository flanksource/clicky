package mcp

import (
	"bytes"
	"strings"
	"testing"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
)

func TestRenderJSONErrorUsesStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	result := mcpsdk.NewToolResultError("failed")
	err := renderCallToolResult(&stdout, &stderr, "demo", result, true)
	if err == nil {
		t.Fatal("error result returned success")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `"isError": true`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
