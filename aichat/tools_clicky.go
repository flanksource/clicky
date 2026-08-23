package aichat

import (
	"context"
	"fmt"
	"sort"
	"strings"

	captools "github.com/flanksource/captain/pkg/ai/tools"
	capchat "github.com/flanksource/captain/pkg/aichat"
	"github.com/flanksource/captain/pkg/api"
	"github.com/flanksource/clicky/entity"
	clickymcp "github.com/flanksource/clicky/mcp"
	"github.com/flanksource/clicky/rpc"
	"github.com/spf13/cobra"
)

type CobraToolFilter func(captools.ToolInfo) bool

type CobraToolProviderOptions struct {
	Root   *cobra.Command
	Filter CobraToolFilter
	// Policy and Strategies are the caller's permission model, carried to captain
	// rather than applied here. This is the seam where a command tree is handed
	// over, and so the place its authority layering is stated: Strategies derive
	// an answer for the tools no rule mentions (nil takes captain's default of
	// HTTP method, then safety hints), and Policy is the ordered rule list —
	// preset first, user last — that overrides them.
	Policy     api.PermissionPolicy
	Strategies []api.PermissionStrategy
}

// CobraToolProvider projects Clicky RPC operations into Captain's provider-
// neutral tool contract. Provider execution and transport remain in Captain.
type CobraToolProvider struct {
	service    *rpc.RPCService
	executor   *rpc.CommandExecutor
	filter     CobraToolFilter
	policy     api.PermissionPolicy
	strategies []api.PermissionStrategy
}

// ToolPolicy and Strategies are the permission model this provider was built
// with, for the caller to pass into captools.ResolveOptions alongside the tool
// set. They are returned rather than applied because resolution belongs to
// captain, which sees every tool source at once.
func (p *CobraToolProvider) ToolPolicy() api.PermissionPolicy { return p.policy }

func (p *CobraToolProvider) Strategies() []api.PermissionStrategy { return p.strategies }

func NewCobraToolProvider(options CobraToolProviderOptions) (*CobraToolProvider, error) {
	if options.Root == nil {
		return nil, fmt.Errorf("Cobra tool provider root command is required")
	}
	config := rpc.DefaultConfig()
	service, err := rpc.NewConverter(config).ConvertCommandTree(options.Root)
	if err != nil {
		return nil, fmt.Errorf("convert command tree: %w", err)
	}
	return &CobraToolProvider{
		service: service,
		executor: rpc.NewCommandExecutor(service, &rpc.ExecutorConfig{
			Enabled: true, SkipPreRun: true, PathPrefix: config.PathPrefix,
		}),
		filter:     options.Filter,
		policy:     options.Policy,
		strategies: options.Strategies,
	}, nil
}

func (p *CobraToolProvider) ToolSet(context.Context) (capchat.ToolSet, error) {
	if p == nil || p.service == nil || p.executor == nil {
		return capchat.ToolSet{}, fmt.Errorf("Cobra tool provider is not initialized")
	}
	set := capchat.ToolSet{}
	seen := map[string]bool{}
	for i := range p.service.Operations {
		op := &p.service.Operations[i]
		if !toolableOperation(op) {
			continue
		}
		name := toolName(op.Name)
		if name == "" {
			return capchat.ToolSet{}, fmt.Errorf("operation %q has no provider-safe tool name", op.Name)
		}
		if seen[name] {
			return capchat.ToolSet{}, fmt.Errorf("operations resolve to duplicate tool name %q", name)
		}
		seen[name] = true
		info, err := toolInfo(name, op)
		if err != nil {
			return capchat.ToolSet{}, err
		}
		if p.filter != nil && !p.filter(info) {
			continue
		}
		schema := jsonSchema(op.Schema)
		definition := api.ToolDefinition{
			Name: name, Description: op.Description, InputSchema: schema,
			Group: info.Group, Parent: info.Parent, Icon: info.Icon,
			DefaultPermission: info.DefaultPermission, Strict: info.Strict,
			ReadOnlyHint: info.ReadOnlyHint, DestructiveHint: info.DestructiveHint,
			IdempotentHint: info.IdempotentHint, Annotations: info.Annotations,
			Operation: info.Operation,
			Handler:   p.handlerFor(op),
		}
		outputSchema, err := rpc.ResponseSchema(*op)
		if err != nil {
			return capchat.ToolSet{}, fmt.Errorf("operation %q response schema: %w", op.Name, err)
		}
		set.Definitions = append(set.Definitions, definition)
		set.Catalog = append(set.Catalog, catalogEntry(definition, outputSchema))
	}
	return set, nil
}

func toolInfo(name string, op *rpc.RPCOperation) (captools.ToolInfo, error) {
	info := captools.ToolInfo{Name: name}
	if op == nil {
		return info, nil
	}
	hints := clickymcp.EffectiveToolHints(op)
	permission, err := declaredToolPermission(op, hints)
	if err != nil {
		return captools.ToolInfo{}, err
	}
	info.Group = hints.Group
	info.Parent = hints.Parent
	info.Icon = hints.Icon
	info.DefaultPermission = permission
	info.Strict = hints.Strict
	info.ReadOnlyHint = hints.ReadOnlyHint
	info.DestructiveHint = hints.DestructiveHint
	info.IdempotentHint = hints.IdempotentHint
	// The operation travels whole. Flattening its entity, verb, scope, method and
	// path into strings here is what obliged captain to parse them back, and left
	// the key names to be kept in step by hand at both ends.
	info.Operation = op
	return info, nil
}

// declaredToolPermission returns the authority an operation's author registered,
// and nothing else. An operation that registered none returns empty, which is
// how it reaches captain's strategies: derived authority is captain's to decide,
// from the method and hints this bridge hands over. Deciding it here is what made
// a consumer's `auto` rule unable to hand a tool back to its own hints, because
// the slot it would defer to had already been filled.
func declaredToolPermission(op *rpc.RPCOperation, hints entity.MCPToolHints) (api.ToolPolicy, error) {
	if hints.DefaultPermission == "" {
		return "", nil
	}
	// Captain owns the mapping, including how the legacy `on` spelling resolves
	// in this encoding. A private copy of that rule here is how the two spellings
	// came to disagree in the first place.
	policy, ok := api.ParseToolPolicy(string(hints.DefaultPermission), api.ParseToolPolicyOptions{
		LegacyOn: api.ToolPolicyAllow,
	})
	if !ok {
		return "", fmt.Errorf("operation %q has invalid default tool permission %q", op.Name, hints.DefaultPermission)
	}
	return policy, nil
}

// catalogEntry builds the frontend-facing DTO. outputSchema is applied raw
// rather than through captools.ObjectSchema: an operation returning an array
// publishes a top-level array, which coercing to an object would corrupt. A nil
// schema leaves the field unset so it drops out of the payload entirely.
func catalogEntry(definition api.ToolDefinition, outputSchema map[string]any) captools.ToolCatalogEntry {
	entry := captools.CustomCatalogEntry(captools.ToolDefinition{
		Name: definition.Name, Description: definition.Description,
		InputSchema: definition.InputSchema, Group: definition.Group,
		Parent: definition.Parent, Icon: definition.Icon,
		DefaultPermission: definition.DefaultPermission, Strict: definition.Strict,
		ReadOnlyHint: definition.ReadOnlyHint, DestructiveHint: definition.DestructiveHint,
		IdempotentHint: definition.IdempotentHint, Annotations: definition.Annotations,
		Operation: definition.Operation,
	}, definition.Name, definition.InputSchema)
	entry.Source = "clicky"
	if outputSchema != nil {
		entry.OutputSchema = outputSchema
	}
	return entry
}

func (p *CobraToolProvider) handlerFor(op *rpc.RPCOperation) api.ToolHandler {
	positional := positionalParams(op)
	return func(ctx context.Context, input map[string]any) (any, error) {
		request := toExecutionRequest(input, positional)
		request.Context = ctx
		data, response, err := p.executor.ExecuteCommand(op, request)
		if err != nil {
			return nil, fmt.Errorf("execute %s: %w", op.Name, err)
		}
		if response != nil && !response.Success {
			return nil, fmt.Errorf("operation %s failed (exit %d): %s", op.Name, response.ExitCode, response.Error)
		}
		return data, nil
	}
}

func toolableOperation(op *rpc.RPCOperation) bool {
	if op == nil || op.Command == nil || !op.Command.Runnable() || op.Command.Hidden() {
		return false
	}
	for _, candidate := range operationCommandNames(op) {
		fields := strings.Fields(strings.ToLower(strings.ReplaceAll(candidate, "/", " ")))
		if len(fields) > 0 && (fields[0] == "completion" || fields[0] == "help") {
			return false
		}
	}
	return true
}

func operationCommandNames(op *rpc.RPCOperation) []string {
	names := []string{op.Name}
	if op.Clicky != nil {
		names = append(names, op.Clicky.Command)
	}
	if op.Command != nil {
		names = append(names, op.Command.Path(), op.Command.Name())
	}
	return names
}

func toolName(raw string) string {
	var name strings.Builder
	for _, char := range raw {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9', char == '_', char == '-':
			name.WriteRune(char)
		default:
			name.WriteByte('_')
		}
	}
	value := name.String()
	if value == "" {
		return ""
	}
	if first := value[0]; first != '_' && !(first >= 'a' && first <= 'z') && !(first >= 'A' && first <= 'Z') {
		value = "_" + value
	}
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

func positionalParams(op *rpc.RPCOperation) []string {
	var names []string
	for _, parameter := range op.Parameters {
		if parameter.In == "path" {
			names = append(names, parameter.Name)
		}
	}
	return names
}

func toExecutionRequest(input map[string]any, positional []string) *rpc.ExecutionRequest {
	request := &rpc.ExecutionRequest{Flags: map[string]string{}}
	positionalSet := make(map[string]bool, len(positional))
	for _, name := range positional {
		positionalSet[name] = true
		if value, ok := input[name]; ok {
			request.Args = append(request.Args, stringify(value))
		}
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !positionalSet[key] {
			request.Flags[key] = stringify(input[key])
		}
	}
	return request
}

func stringify(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprintf("%v", value)
}

func jsonSchema(schema rpc.Schema) map[string]any {
	properties := map[string]any{}
	for name, property := range schema.Properties {
		entry := map[string]any{"type": property.Type}
		if property.Type == "array" {
			entry["items"] = map[string]any{"type": "string"}
		}
		if property.Description != "" {
			entry["description"] = property.Description
		}
		if len(property.Enum) > 0 {
			entry["enum"] = append([]string(nil), property.Enum...)
		}
		if property.Default != nil {
			entry["default"] = property.Default
		}
		properties[name] = entry
	}
	result := map[string]any{"type": "object", "properties": properties}
	if len(schema.Required) > 0 {
		result["required"] = append([]string(nil), schema.Required...)
	}
	return result
}
