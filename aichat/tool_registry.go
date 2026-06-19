package aichat

import (
	"context"
	"fmt"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

type registeredTool struct {
	ref  ai.ToolRef
	info ToolInfo
}

type toolRuntimeConfig struct {
	preferences     ToolPreferences
	defaultApproval approvalPredicate
}

type toolRuntimeContextKey struct{}

// ToolDefinition describes an app-owned tool registered alongside clicky RPC
// and MCP tools. Handlers should return JSON-serializable values.
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]any
	Method      string
	Path        string
	Verb        string
	Scope       string
	Handler     func(context.Context, any) (any, error)
}

func DefineCustomTools(g *genkit.Genkit, defs []ToolDefinition) ([]registeredTool, error) {
	if len(defs) == 0 {
		return nil, nil
	}
	out := make([]registeredTool, 0, len(defs))
	seen := map[string]bool{}
	for _, def := range defs {
		name := toolName(def.Name)
		if name == "" {
			return nil, fmt.Errorf("custom tool %q has no valid name", def.Name)
		}
		if def.Handler == nil {
			return nil, fmt.Errorf("custom tool %q has no handler", def.Name)
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate custom tool %q", name)
		}
		seen[name] = true
		schema := def.InputSchema
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		info := ToolInfo{
			Name:          name,
			OperationName: def.Name,
			Method:        def.Method,
			Path:          def.Path,
			ClickyVerb:    def.Verb,
			ClickyScope:   def.Scope,
		}
		tool := genkit.DefineTool[any, any](g, name, def.Description,
			func(tc *ai.ToolContext, input any) (any, error) {
				if !tc.IsResumed() && shouldRequireApproval(tc.Context, nil, info, input) {
					return nil, interruptForApproval(tc, name)
				}
				return def.Handler(tc.Context, input)
			},
			ai.WithInputSchema(schema),
		)
		out = append(out, registeredTool{ref: tool, info: info})
	}
	return out, nil
}

func withToolRuntime(ctx context.Context, cfg toolRuntimeConfig) context.Context {
	return context.WithValue(ctx, toolRuntimeContextKey{}, cfg)
}

func toolRuntime(ctx context.Context) (toolRuntimeConfig, bool) {
	cfg, ok := ctx.Value(toolRuntimeContextKey{}).(toolRuntimeConfig)
	return cfg, ok
}

func toolRefs(tools []registeredTool) []ai.ToolRef {
	if len(tools) == 0 {
		return nil
	}
	refs := make([]ai.ToolRef, 0, len(tools))
	for _, tool := range tools {
		refs = append(refs, tool.ref)
	}
	return refs
}

func toolsForRequest(tools []registeredTool, prefs ToolPreferences) []ai.ToolRef {
	if len(tools) == 0 {
		return nil
	}
	refs := make([]ai.ToolRef, 0, len(tools))
	for _, tool := range tools {
		if mode, ok := normalizedPreference(prefs, tool.info.Name); ok && mode == ToolModeDisabled {
			continue
		}
		refs = append(refs, tool.ref)
	}
	return refs
}

func normalizedPreference(prefs ToolPreferences, name string) (ToolMode, bool) {
	if len(prefs) == 0 {
		return "", false
	}
	mode, ok := prefs[name]
	if !ok {
		return "", false
	}
	return normalizeToolMode(mode)
}

func shouldRequireApproval(ctx context.Context, fallback approvalPredicate, tool ToolInfo, input any) bool {
	if ctx != nil {
		if cfg, ok := toolRuntime(ctx); ok {
			if mode, ok := normalizedPreference(cfg.preferences, tool.Name); ok {
				switch mode {
				case ToolModeEnabled:
					return false
				case ToolModeAsk:
					return true
				case ToolModeDisabled:
					return false
				}
			}
			if cfg.defaultApproval != nil {
				return cfg.defaultApproval(tool, input)
			}
		}
	}
	if fallback == nil {
		return false
	}
	return fallback(tool, input)
}
