package aichat

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/flanksource/clicky/rpc"
)

type ToolCatalog struct {
	Tools []ToolCatalogEntry `json:"tools"`
}

type ToolCatalogEntry struct {
	Name              string         `json:"name"`
	Title             string         `json:"title,omitempty"`
	Description       string         `json:"description,omitempty"`
	Source            string         `json:"source"`
	Server            string         `json:"server,omitempty"`
	Group             string         `json:"group,omitempty"`
	Parent            string         `json:"parent,omitempty"`
	Icon              string         `json:"icon,omitempty"`
	PreferenceKey     string         `json:"preferenceKey,omitempty"`
	DefaultPermission ToolMode       `json:"defaultPermission,omitempty"`
	Strict            *bool          `json:"strict,omitempty"`
	Method            string         `json:"method,omitempty"`
	Path              string         `json:"path,omitempty"`
	OperationName     string         `json:"operationName,omitempty"`
	InputSchema       map[string]any `json:"inputSchema"`
	OutputSchema      map[string]any `json:"outputSchema,omitempty"`
}

type toolWithDefinition interface {
	Definition() *ai.ToolDefinition
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	tools, err := s.toolsForCatalog(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(toolCatalog(tools)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) toolsForCatalog(ctx context.Context) ([]registeredTool, error) {
	g := genkit.Init(ctx)
	return s.buildTools(ctx, g)
}

func toolCatalog(tools []registeredTool) ToolCatalog {
	entries := make([]ToolCatalogEntry, 0, len(tools))
	for _, tool := range tools {
		if tool.catalog != nil {
			entries = append(entries, *tool.catalog)
			continue
		}
		entries = append(entries, catalogEntryFromToolRef("custom", tool.ref, tool.info))
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Source != entries[j].Source {
			return entries[i].Source < entries[j].Source
		}
		if entries[i].Group != entries[j].Group {
			return entries[i].Group < entries[j].Group
		}
		return entries[i].Name < entries[j].Name
	})
	return ToolCatalog{Tools: entries}
}

func clickyCatalogEntry(name string, op *rpc.RPCOperation, schema map[string]any) ToolCatalogEntry {
	info := toolInfo(name, op)
	entry := ToolCatalogEntry{
		Name:              name,
		Title:             info.OperationName,
		Source:            "clicky",
		Group:             info.Group,
		Parent:            info.Parent,
		Icon:              info.Icon,
		PreferenceKey:     preferenceKey(info),
		DefaultPermission: defaultPermissionMode(info.DefaultPermission),
		Strict:            info.Strict,
		Method:            info.Method,
		Path:              info.Path,
		OperationName:     info.OperationName,
		InputSchema:       objectSchema(schema),
	}
	if op != nil {
		entry.Description = op.Description
	}
	return entry
}

func customCatalogEntry(def ToolDefinition, name string, schema map[string]any) ToolCatalogEntry {
	info := ToolInfo{
		Name:              name,
		OperationName:     def.Name,
		Method:            def.Method,
		Path:              def.Path,
		ClickyVerb:        def.Verb,
		ClickyScope:       def.Scope,
		Group:             def.Group,
		Parent:            def.Parent,
		Icon:              def.Icon,
		DefaultPermission: defaultPermissionMode(def.DefaultPermission),
		Strict:            def.Strict,
	}
	return ToolCatalogEntry{
		Name:              name,
		Title:             def.Name,
		Description:       def.Description,
		Source:            "custom",
		Group:             def.Group,
		Parent:            def.Parent,
		Icon:              def.Icon,
		PreferenceKey:     preferenceKey(info),
		DefaultPermission: defaultPermissionMode(def.DefaultPermission),
		Strict:            def.Strict,
		Method:            def.Method,
		Path:              def.Path,
		OperationName:     def.Name,
		InputSchema:       objectSchema(schema),
	}
}

func catalogEntryFromToolRef(source string, ref ai.ToolRef, info ToolInfo) ToolCatalogEntry {
	entry := ToolCatalogEntry{
		Name:              ref.Name(),
		Source:            source,
		Group:             info.Group,
		Parent:            info.Parent,
		Icon:              info.Icon,
		PreferenceKey:     preferenceKey(info),
		DefaultPermission: defaultPermissionMode(info.DefaultPermission),
		Strict:            info.Strict,
		Method:            info.Method,
		Path:              info.Path,
		OperationName:     info.OperationName,
		InputSchema:       objectSchema(nil),
	}
	if withDef, ok := ref.(toolWithDefinition); ok {
		if def := withDef.Definition(); def != nil {
			if def.Name != "" {
				entry.Name = def.Name
			}
			entry.Title = def.Name
			entry.Description = def.Description
			entry.InputSchema = objectSchema(def.InputSchema)
			entry.OutputSchema = def.OutputSchema
			if server, ok := stringMetadata(def.Metadata, "mcp_server", "server", "serverName"); ok {
				entry.Server = server
			}
			applyToolMetadata(&entry, def.Metadata)
		}
	}
	if entry.PreferenceKey == "" {
		entry.PreferenceKey = entry.Name
	}
	if entry.OperationName == "" {
		entry.OperationName = entry.Name
	}
	return entry
}

func preferenceKey(info ToolInfo) string {
	if info.Group != "" {
		return info.Group
	}
	return info.Name
}

func objectSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	if _, ok := schema["type"]; !ok {
		schema["type"] = "object"
	}
	if schema["type"] == "object" {
		if _, ok := schema["properties"]; !ok {
			schema["properties"] = map[string]any{}
		}
	}
	return schema
}

func stringMetadata(meta map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if v, ok := meta[key].(string); ok && v != "" {
			return v, true
		}
	}
	return "", false
}

func applyToolMetadata(entry *ToolCatalogEntry, meta map[string]any) {
	if entry == nil || len(meta) == 0 {
		return
	}
	if group, ok := stringMetadata(meta, "group", "toolGroup"); ok && entry.Group == "" {
		entry.Group = group
		entry.PreferenceKey = group
	}
	if parent, ok := stringMetadata(meta, "parent", "toolParent"); ok && entry.Parent == "" {
		entry.Parent = parent
	}
	if icon, ok := stringMetadata(meta, "icon", "toolIcon"); ok && entry.Icon == "" {
		entry.Icon = icon
	}
	if permission, ok := stringMetadata(meta, "defaultPermission", "permission", "defaultMode"); ok {
		entry.DefaultPermission = defaultPermissionMode(ToolMode(permission))
	}
	if strict, ok := boolMetadata(meta, "strict"); ok && entry.Strict == nil {
		entry.Strict = &strict
	}
	if nested, ok := meta["com.flanksource.clicky/tool"].(map[string]any); ok {
		applyToolMetadata(entry, nested)
	}
}

func boolMetadata(meta map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		switch v := meta[key].(type) {
		case bool:
			return v, true
		case string:
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "true":
				return true, true
			case "false":
				return false, true
			}
		}
	}
	return false, false
}
