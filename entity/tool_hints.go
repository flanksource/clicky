package entity

// ToolPermission is the default approval mode a tool advertises to clients.
type ToolPermission string

const (
	ToolPermissionAsk  ToolPermission = "ask"
	ToolPermissionOff  ToolPermission = "off"
	ToolPermissionOn   ToolPermission = "on"
	ToolPermissionAuto ToolPermission = "auto"
)

// MCPToolHints describes MCP tool annotations plus Clicky-specific UI and
// approval metadata. Boolean fields are pointers because both true and false
// are meaningful MCP hint values.
type MCPToolHints struct {
	Title             string         `json:"title,omitempty"`
	ReadOnlyHint      *bool          `json:"readOnlyHint,omitempty"`
	DestructiveHint   *bool          `json:"destructiveHint,omitempty"`
	IdempotentHint    *bool          `json:"idempotentHint,omitempty"`
	OpenWorldHint     *bool          `json:"openWorldHint,omitempty"`
	Icon              string         `json:"icon,omitempty"`
	Group             string         `json:"group,omitempty"`
	Parent            string         `json:"parent,omitempty"`
	DefaultPermission ToolPermission `json:"defaultPermission,omitempty"`
	Strict            *bool          `json:"strict,omitempty"`
}

func (h MCPToolHints) isZero() bool {
	return h.Title == "" &&
		h.ReadOnlyHint == nil &&
		h.DestructiveHint == nil &&
		h.IdempotentHint == nil &&
		h.OpenWorldHint == nil &&
		h.Icon == "" &&
		h.Group == "" &&
		h.Parent == "" &&
		h.DefaultPermission == "" &&
		h.Strict == nil
}

// IsZero reports whether no tool metadata or MCP annotations are set.
func (h MCPToolHints) IsZero() bool {
	return h.isZero()
}

func (h MCPToolHints) merge(override MCPToolHints) MCPToolHints {
	if override.Title != "" {
		h.Title = override.Title
	}
	if override.ReadOnlyHint != nil {
		h.ReadOnlyHint = override.ReadOnlyHint
	}
	if override.DestructiveHint != nil {
		h.DestructiveHint = override.DestructiveHint
	}
	if override.IdempotentHint != nil {
		h.IdempotentHint = override.IdempotentHint
	}
	if override.OpenWorldHint != nil {
		h.OpenWorldHint = override.OpenWorldHint
	}
	if override.Icon != "" {
		h.Icon = override.Icon
	}
	if override.Group != "" {
		h.Group = override.Group
	}
	if override.Parent != "" {
		h.Parent = override.Parent
	}
	if override.DefaultPermission != "" {
		h.DefaultPermission = override.DefaultPermission
	}
	if override.Strict != nil {
		h.Strict = override.Strict
	}
	return h
}

func normalizeToolPermission(permission ToolPermission) ToolPermission {
	switch permission {
	case ToolPermissionAsk, ToolPermissionOff, ToolPermissionOn, ToolPermissionAuto:
		return permission
	default:
		return ""
	}
}
