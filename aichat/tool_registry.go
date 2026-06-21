package aichat

import (
	"context"
	"fmt"
	"sort"

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
	// Group, when set, places this custom tool in a tool-group so the
	// preferences UI presents it under the group rather than individually.
	Group   string
	Handler func(context.Context, any) (any, error)
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
			Group:         def.Group,
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
		if mode, ok := effectivePreference(prefs, tool.info); ok && mode == ToolModeDisabled {
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

// effectivePreference resolves the ToolMode for a tool. A tool that belongs to a
// group is governed solely by the group's preference — the preferences UI hides
// the individual members, so a member-name key is ignored when a group is set.
// Ungrouped tools resolve by their own name.
func effectivePreference(prefs ToolPreferences, info ToolInfo) (ToolMode, bool) {
	if info.Group != "" {
		return normalizedPreference(prefs, info.Group)
	}
	return normalizedPreference(prefs, info.Name)
}

// ToolEntry is one row in the tool-preferences UI: either a single ungrouped
// tool (Group=="", Tools holds its one name) or a collapsed group (Group!="",
// Tools lists the member tool names). Mode is the current preference, if set.
type ToolEntry struct {
	Key   string   `json:"key"`             // group name, or tool name when ungrouped
	Group string   `json:"group,omitempty"` // non-empty for group entries
	Tools []string `json:"tools"`           // member tool names
	Mode  ToolMode `json:"mode,omitempty"`  // current preference, if set
}

// ListToolEntries collapses grouped tools into a single entry per group (hiding
// the individual members) and leaves ungrouped tools as individual entries.
// Entries are sorted by Key for a deterministic UI. prefs (may be nil) annotates
// each entry's current Mode.
func ListToolEntries(tools []registeredTool, prefs ToolPreferences) []ToolEntry {
	groups := map[string][]string{}
	var entries []ToolEntry
	for _, tool := range tools {
		if g := tool.info.Group; g != "" {
			groups[g] = append(groups[g], tool.info.Name)
			continue
		}
		entry := ToolEntry{Key: tool.info.Name, Tools: []string{tool.info.Name}}
		if mode, ok := normalizedPreference(prefs, tool.info.Name); ok {
			entry.Mode = mode
		}
		entries = append(entries, entry)
	}
	for group, members := range groups {
		sort.Strings(members)
		entry := ToolEntry{Key: group, Group: group, Tools: members}
		if mode, ok := normalizedPreference(prefs, group); ok {
			entry.Mode = mode
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return entries
}

func shouldRequireApproval(ctx context.Context, fallback approvalPredicate, tool ToolInfo, input any) bool {
	if ctx != nil {
		if cfg, ok := toolRuntime(ctx); ok {
			if mode, ok := effectivePreference(cfg.preferences, tool); ok {
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
