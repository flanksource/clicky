package aichat

import (
	"context"
	"fmt"
	"testing"

	"github.com/firebase/genkit/go/ai"
)

func boolPointer(v bool) *bool { return &v }

type strictTestTool string

func (t strictTestTool) Name() string { return string(t) }

func TestStrictToolCandidatesIncludeOnlyExplicitOptIns(t *testing.T) {
	tools := []registeredTool{
		{info: ToolInfo{Name: "readonly", ReadOnlyHint: boolPointer(true)}},
		{info: ToolInfo{Name: "unknown"}},
		{info: ToolInfo{Name: "mutating", ReadOnlyHint: boolPointer(false)}},
		{info: ToolInfo{Name: "non_idempotent", IdempotentHint: boolPointer(false)}},
		{info: ToolInfo{Name: "destructive", DestructiveHint: boolPointer(true)}},
		{info: ToolInfo{Name: "explicit", Strict: boolPointer(true), ReadOnlyHint: boolPointer(true)}},
		{info: ToolInfo{Name: "loose", Strict: boolPointer(false), DestructiveHint: boolPointer(true)}},
	}

	candidates := strictToolCandidates(tools)
	want := []string{"explicit"}
	if len(candidates) != len(want) {
		t.Fatalf("candidates = %d, want %d", len(candidates), len(want))
	}
	for i, name := range want {
		if candidates[i].info.Name != name {
			t.Fatalf("candidate[%d] = %q, want %q", i, candidates[i].info.Name, name)
		}
	}
}

func TestAnthropicStrictToolsMiddlewareCapsWithoutRemovingTools(t *testing.T) {
	const toolCount = 54
	tools := make([]registeredTool, 0, toolCount)
	defs := make([]*ai.ToolDefinition, 0, toolCount)
	for i := range toolCount {
		name := fmt.Sprintf("readonly_%02d", i)
		info := ToolInfo{Name: name, Strict: boolPointer(true), ReadOnlyHint: boolPointer(true)}
		if i == toolCount-1 {
			name = "destructive"
			info = ToolInfo{Name: name, Strict: boolPointer(true), DestructiveHint: boolPointer(true)}
		}
		tools = append(tools, registeredTool{info: info})
		defs = append(defs, &ai.ToolDefinition{Name: name})
	}

	warnings := 0
	mw := anthropicStrictToolsMiddleware(tools, func(string, ...any) { warnings++ })
	if mw == nil {
		t.Fatal("middleware = nil, want strict-tool limiter")
	}
	var got *ai.ModelRequest
	next := func(_ context.Context, req *ai.ModelRequest, _ ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		got = req
		return &ai.ModelResponse{}, nil
	}
	wrapped := mw(next)
	req := &ai.ModelRequest{Tools: defs}
	if _, err := wrapped(context.Background(), req, nil); err != nil {
		t.Fatalf("middleware: %v", err)
	}
	if _, err := wrapped(context.Background(), req, nil); err != nil {
		t.Fatalf("middleware second call: %v", err)
	}

	if warnings != 1 {
		t.Fatalf("warnings = %d, want 1", warnings)
	}
	if len(got.Tools) != len(defs) {
		t.Fatalf("tools = %d, want all %d tools retained", len(got.Tools), len(defs))
	}
	strict := 0
	for _, def := range got.Tools {
		if value, _ := def.Metadata["strict"].(bool); value {
			strict++
		}
	}
	if strict != anthropicMaxStrictTools {
		t.Fatalf("strict tools = %d, want %d", strict, anthropicMaxStrictTools)
	}
	for _, def := range got.Tools {
		if def.Name == "destructive" {
			if value, _ := def.Metadata["strict"].(bool); !value {
				t.Fatal("destructive tool was demoted behind read-only tools")
			}
		}
	}
	if defs[0].Metadata != nil {
		t.Fatal("middleware mutated the shared/original tool definition")
	}
}

func TestAnthropicStrictToolsMiddlewareDefaultsEveryToolToNonStrict(t *testing.T) {
	tools := []registeredTool{
		{info: ToolInfo{Name: "unset"}},
		{info: ToolInfo{Name: "explicit_false", Strict: boolPointer(false)}},
		{info: ToolInfo{Name: "explicit_true", Strict: boolPointer(true)}},
	}
	defs := []*ai.ToolDefinition{{Name: "unset"}, {Name: "explicit_false"}, {Name: "explicit_true"}}

	mw := anthropicStrictToolsMiddleware(tools, nil)
	if mw == nil {
		t.Fatal("middleware = nil, want strict opt-in metadata middleware")
	}
	var got *ai.ModelRequest
	next := func(_ context.Context, req *ai.ModelRequest, _ ai.ModelStreamCallback) (*ai.ModelResponse, error) {
		got = req
		return &ai.ModelResponse{}, nil
	}
	if _, err := mw(next)(context.Background(), &ai.ModelRequest{Tools: defs}, nil); err != nil {
		t.Fatalf("middleware: %v", err)
	}

	for _, def := range got.Tools {
		strict, ok := def.Metadata["strict"].(bool)
		if !ok {
			t.Fatalf("tool %q strict metadata = %#v, want bool", def.Name, def.Metadata["strict"])
		}
		if strict != (def.Name == "explicit_true") {
			t.Fatalf("tool %q strict = %v, want opt-in behavior", def.Name, strict)
		}
	}
	if defs[0].Metadata != nil {
		t.Fatal("middleware mutated the shared/original tool definition")
	}
}

func TestAnthropicStrictToolsMiddlewareCountsAfterPreferences(t *testing.T) {
	tools := make([]registeredTool, 0, 21)
	for i := range 21 {
		name := fmt.Sprintf("tool_%02d", i)
		tools = append(tools, registeredTool{ref: strictTestTool(name), info: ToolInfo{Name: name, Strict: boolPointer(true)}})
	}
	selected := registeredToolsForRequest(tools, ToolPreferences{"tool_20": ToolModeOff})
	if len(selected) != anthropicMaxStrictTools {
		t.Fatalf("selected tools = %d, want %d", len(selected), anthropicMaxStrictTools)
	}
	if candidates := strictToolCandidates(selected); len(candidates) != anthropicMaxStrictTools {
		t.Fatalf("strict candidates = %d, want %d after preferences", len(candidates), anthropicMaxStrictTools)
	}
}
