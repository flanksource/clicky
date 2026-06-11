package aichat

import (
	"regexp"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/rpc"
	"github.com/spf13/cobra"
)

type greetOpts struct {
	Name string `flag:"name" help:"who to greet"`
}

type greetResult struct {
	Message string `json:"message"`
}

// buildToolset wires a trivial cobra tree (one `greet` command backed by a
// DataFunc) through the clicky converter + in-process executor.
func buildToolset(t *testing.T) *ClickyToolset {
	t.Helper()
	root := &cobra.Command{Use: "test"}
	clicky.AddCommand(root, greetOpts{}, func(o greetOpts) (greetResult, error) {
		return greetResult{Message: "hello " + o.Name}, nil
	})
	ts, err := NewClickyToolset(root)
	if err != nil {
		t.Fatalf("NewClickyToolset: %v", err)
	}
	return ts
}

func findOp(t *testing.T, ts *ClickyToolset, name string) *rpc.RPCOperation {
	t.Helper()
	for i := range ts.service.Operations {
		if ts.service.Operations[i].Name == name {
			return &ts.service.Operations[i]
		}
	}
	t.Fatalf("operation %q not found; have %v", name, opNames(ts))
	return nil
}

func opNames(ts *ClickyToolset) []string {
	var names []string
	for i := range ts.service.Operations {
		names = append(names, ts.service.Operations[i].Name)
	}
	return names
}

func TestClickyToolsetExecutesInProcess(t *testing.T) {
	ts := buildToolset(t)
	op := findOp(t, ts, "greet-opts")

	handler := ts.handlerFor(op)
	out, err := handler(&ai.ToolContext{}, map[string]any{"name": "world"})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}

	res, ok := out.(greetResult)
	if !ok {
		t.Fatalf("output type = %T, want greetResult", out)
	}
	if res.Message != "hello world" {
		t.Errorf("message = %q, want %q", res.Message, "hello world")
	}
}

func TestJSONSchemaConversion(t *testing.T) {
	s := rpc.Schema{
		Type: "object",
		Properties: map[string]rpc.Property{
			"name": {Type: "string", Description: "who"},
		},
		Required: []string{"name"},
	}
	got := jsonSchema(s)
	if got["type"] != "object" {
		t.Errorf("type = %v, want object", got["type"])
	}
	props, ok := got["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties type = %T", got["properties"])
	}
	name, ok := props["name"].(map[string]any)
	if !ok {
		t.Fatalf("name prop type = %T", props["name"])
	}
	if name["type"] != "string" || name["description"] != "who" {
		t.Errorf("name prop = %v", name)
	}
	req, ok := got["required"].([]any)
	if !ok || len(req) != 1 || req[0] != "name" {
		t.Errorf("required = %v", got["required"])
	}
}

func TestToolNameSanitizes(t *testing.T) {
	cases := map[string]string{
		"stack get":             "stack_get",
		"completion powershell": "completion_powershell",
		"admin stack reconcile": "admin_stack_reconcile",
		"already-valid.name":    "already-valid.name",
		"123leading":            "_123leading",
		"":                      "",
	}
	for in, want := range cases {
		if got := toolName(in); got != want {
			t.Errorf("toolName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToolNameMatchesProviderCharset(t *testing.T) {
	// Gemini/AI-SDK require ^[A-Za-z_][A-Za-z0-9_.-]{0,63}$.
	valid := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]{0,63}$`)
	for _, raw := range []string{"stack get", "completion powershell", "weird/name:v2"} {
		if got := toolName(raw); !valid.MatchString(got) {
			t.Errorf("toolName(%q) = %q is not a valid provider tool name", raw, got)
		}
	}
}

func TestJSONSchemaArrayGetsItems(t *testing.T) {
	got := jsonSchema(rpc.Schema{
		Type: "object",
		Properties: map[string]rpc.Property{
			"args": {Type: "array", Description: "Positional arguments"},
		},
	})
	props := got["properties"].(map[string]any)
	args := props["args"].(map[string]any)
	items, ok := args["items"].(map[string]any)
	if !ok {
		t.Fatalf("array property missing items: %v", args)
	}
	if items["type"] != "string" {
		t.Errorf("items type = %v, want string", items["type"])
	}
}

func TestToExecutionRequestSplitsPositional(t *testing.T) {
	req := toExecutionRequest(
		map[string]any{"id": "abc", "verbose": true},
		[]string{"id"},
	)
	if len(req.Args) != 1 || req.Args[0] != "abc" {
		t.Errorf("args = %v, want [abc]", req.Args)
	}
	if req.Flags["verbose"] != "true" {
		t.Errorf("flags[verbose] = %q, want true", req.Flags["verbose"])
	}
	if _, ok := req.Flags["id"]; ok {
		t.Errorf("positional id leaked into flags: %v", req.Flags)
	}
}
