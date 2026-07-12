package aichat

import (
	"context"
	"regexp"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
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

func TestClickyToolCatalogPreservesInputSchema(t *testing.T) {
	ts := buildToolset(t)
	g := genkit.Init(context.Background())
	tools := ts.DefineRegisteredTools(g)
	catalog := toolCatalog(tools)
	if len(catalog.Tools) != 1 {
		t.Fatalf("tools = %d, want 1", len(catalog.Tools))
	}
	tool := catalog.Tools[0]
	if tool.Source != "clicky" || tool.OperationName != "greet-opts" {
		t.Fatalf("tool = %+v, want clicky greet-opts", tool)
	}
	props := tool.InputSchema["properties"].(map[string]any)
	name := props["name"].(map[string]any)
	if name["type"] != "string" || name["description"] == "" {
		t.Fatalf("name schema = %v", name)
	}
	withDefinition, ok := tools[0].ref.(toolWithDefinition)
	if !ok {
		t.Fatalf("tool ref %T does not expose its Genkit definition", tools[0].ref)
	}
	if strict, ok := withDefinition.Definition().Metadata["strict"].(bool); !ok || strict {
		t.Fatalf("Genkit strict metadata = %v, want default false", withDefinition.Definition().Metadata["strict"])
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

func TestToolableOperationSkipsCobraHelpAndCompletion(t *testing.T) {
	runnable := func(use string) *cobra.Command {
		return &cobra.Command{Use: use, Run: func(cmd *cobra.Command, args []string) {}}
	}
	cases := []struct {
		name string
		op   *rpc.RPCOperation
		want bool
	}{
		{
			name: "operation name completion",
			op: &rpc.RPCOperation{
				Name:    "completion bash",
				Command: rpc.NewCobraExecutableCommand(runnable("bash")),
			},
			want: false,
		},
		{
			name: "operation name help",
			op: &rpc.RPCOperation{
				Name:    "help accounts",
				Command: rpc.NewCobraExecutableCommand(runnable("accounts")),
			},
			want: false,
		},
		{
			name: "x-clicky command completion",
			op: &rpc.RPCOperation{
				Name:    "bash",
				Command: rpc.NewCobraExecutableCommand(runnable("bash")),
				Clicky:  &rpc.ClickyOperationMeta{Command: "completion/bash"},
			},
			want: false,
		},
		{
			name: "leaf help command",
			op: &rpc.RPCOperation{
				Name:    "docs",
				Command: rpc.NewCobraExecutableCommand(runnable("help")),
			},
			want: false,
		},
		{
			name: "ordinary helpful command",
			op: &rpc.RPCOperation{
				Name:    "helpful report",
				Command: rpc.NewCobraExecutableCommand(runnable("helpful")),
			},
			want: true,
		},
		{
			// A hyphen is part of the command name, not a path separator: "help-desk"
			// must not be mistaken for the built-in `help` command.
			name: "hyphenated command is not the help builtin",
			op: &rpc.RPCOperation{
				Name:    "help-desk",
				Command: rpc.NewCobraExecutableCommand(runnable("help-desk")),
			},
			want: true,
		},
		{
			name: "underscored command is not the completion builtin",
			op: &rpc.RPCOperation{
				Name:    "completion_report",
				Command: rpc.NewCobraExecutableCommand(runnable("completion_report")),
			},
			want: true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := toolableOperation(tt.op); got != tt.want {
				t.Fatalf("toolableOperation() = %v, want %v", got, tt.want)
			}
		})
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
