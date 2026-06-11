package aichat

import (
	"fmt"
	"sort"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/flanksource/clicky/rpc"
	"github.com/spf13/cobra"
)

// ClickyToolset adapts a Cobra command tree into Genkit tools backed by clicky's
// in-process RPC executor. Operations are discovered once via the converter; each
// becomes a dynamic Genkit tool whose input schema is the operation's JSON schema.
type ClickyToolset struct {
	service  *rpc.RPCService
	executor *rpc.CommandExecutor
	// requireApproval, when non-nil, gates a tool behind user approval before it
	// executes (human-in-the-loop). nil means every tool runs automatically.
	requireApproval approvalPredicate
}

// NewClickyToolset converts rootCmd into an RPC service and wires an enabled
// in-process executor. SkipPreRun avoids re-running cobra root hooks per call.
func NewClickyToolset(rootCmd *cobra.Command) (*ClickyToolset, error) {
	service, err := rpc.NewConverter(rpc.DefaultConfig()).ConvertCommandTree(rootCmd)
	if err != nil {
		return nil, fmt.Errorf("convert command tree: %w", err)
	}
	executor := rpc.NewCommandExecutor(service, &rpc.ExecutorConfig{
		Enabled:    true,
		SkipPreRun: true,
		PathPrefix: rpc.DefaultConfig().PathPrefix,
	})
	return &ClickyToolset{service: service, executor: executor}, nil
}

// DefineTools registers each runnable operation as a Genkit tool on g and
// returns them as ToolRefs ready for ai.WithTools. Non-runnable group commands
// and cobra built-ins (completion/help) are skipped; operation names are
// sanitized into the provider-safe identifier charset.
func (t *ClickyToolset) DefineTools(g *genkit.Genkit) []ai.ToolRef {
	refs := make([]ai.ToolRef, 0, len(t.service.Operations))
	seen := map[string]bool{}
	for i := range t.service.Operations {
		op := &t.service.Operations[i]
		if !toolableOperation(op) {
			continue
		}
		name := toolName(op.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		schema := jsonSchema(op.Schema)
		tool := genkit.DefineTool[any, any](g, name, op.Description,
			t.handlerFor(op),
			ai.WithInputSchema(schema),
		)
		refs = append(refs, tool)
	}
	return refs
}

// toolableOperation reports whether an operation should be exposed to the model.
// Pure grouping commands (no Run), hidden commands, and cobra's auto-generated
// completion/help trees are not useful tools.
func toolableOperation(op *rpc.RPCOperation) bool {
	cmd := op.Command
	if cmd == nil || !cmd.Runnable() || cmd.Hidden {
		return false
	}
	if fields := strings.Fields(op.Name); len(fields) > 0 {
		switch fields[0] {
		case "completion", "help":
			return false
		}
	}
	return true
}

// toolName converts a clicky operation name ("stack get") into a provider-safe
// tool name: letters, digits, '_', '.', '-', starting with a letter or
// underscore, capped at 64 chars. AI providers reject spaces and other
// characters, so the raw space-joined command path cannot be used directly.
func toolName(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '_', r == '.', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	name := b.String()
	if name == "" {
		return ""
	}
	if c := name[0]; c != '_' && !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') {
		name = "_" + name
	}
	if len(name) > 64 {
		name = name[:64]
	}
	return name
}

// handlerFor returns the execute function for one operation. Model input arrives
// as map[string]any (per the schema); we split it into clicky positional args
// and string flags, then execute in-process.
func (t *ClickyToolset) handlerFor(op *rpc.RPCOperation) ai.ToolFunc[any, any] {
	positional := positionalParams(op)
	name := toolName(op.Name)
	return func(tc *ai.ToolContext, input any) (any, error) {
		// First pass for a gated tool: pause for user approval. On resume
		// (tc.IsResumed) we fall through and execute the approved call.
		if !tc.IsResumed() && t.requireApproval != nil && t.requireApproval(name, input) {
			return nil, interruptForApproval(tc, name)
		}
		req := toExecutionRequest(input, positional)
		data, resp, err := t.executor.ExecuteCommand(op, req)
		if err != nil {
			return nil, fmt.Errorf("execute %s: %w", op.Name, err)
		}
		if resp != nil && !resp.Success {
			return nil, fmt.Errorf("operation %s failed (exit %d): %s", op.Name, resp.ExitCode, resp.Error)
		}
		return data, nil
	}
}

// positionalParams returns the parameter names declared "in: path" — these map
// to clicky positional Args rather than Flags.
func positionalParams(op *rpc.RPCOperation) []string {
	var names []string
	for _, p := range op.Parameters {
		if p.In == "path" {
			names = append(names, p.Name)
		}
	}
	return names
}

// toExecutionRequest splits a model-provided input map into positional Args
// (in declared order) and the remaining keys as string Flags.
func toExecutionRequest(input any, positional []string) *rpc.ExecutionRequest {
	m, _ := input.(map[string]any)
	req := &rpc.ExecutionRequest{Flags: map[string]string{}}
	if m == nil {
		return req
	}
	isPositional := make(map[string]bool, len(positional))
	for _, name := range positional {
		isPositional[name] = true
	}
	for _, name := range positional {
		if v, ok := m[name]; ok {
			req.Args = append(req.Args, stringify(v))
		}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if isPositional[k] {
			continue
		}
		req.Flags[k] = stringify(m[k])
	}
	return req
}

func stringify(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", x)
	}
}

// jsonSchema converts clicky's rpc.Schema into the generic JSON-Schema map that
// ai.WithInputSchema expects.
func jsonSchema(s rpc.Schema) map[string]any {
	props := map[string]any{}
	for name, p := range s.Properties {
		entry := map[string]any{"type": p.Type}
		// JSON-Schema requires `items` for arrays, and providers (e.g. Gemini)
		// reject array properties without it. clicky's Property has no item
		// type; its array params are variadic positional args, which are
		// strings, so default to a string element schema.
		if p.Type == "array" {
			entry["items"] = map[string]any{"type": "string"}
		}
		if p.Description != "" {
			entry["description"] = p.Description
		}
		if len(p.Enum) > 0 {
			enum := make([]any, len(p.Enum))
			for i, e := range p.Enum {
				enum[i] = e
			}
			entry["enum"] = enum
		}
		if p.Default != nil {
			entry["default"] = p.Default
		}
		props[name] = entry
	}
	schema := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(s.Required) > 0 {
		req := make([]any, len(s.Required))
		for i, r := range s.Required {
			req[i] = r
		}
		schema["required"] = req
	}
	return schema
}
