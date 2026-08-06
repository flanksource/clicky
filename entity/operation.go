package entity

import "reflect"

// DataFunc is a function that returns structured data directly, bypassing stdout
// capture. Used by commands registered via AddCommand to provide data to the HTTP
// handler. (ContextDataFunc / ContextLookupFunc — the context-aware variants —
// are defined alongside the command registry in command.go.)
type DataFunc func(flags map[string]string, args []string) (any, error)

// RPCOperation represents a generic RPC operation that can be converted to
// various formats (OpenAPI, MCP tools, …).
type RPCOperation struct {
	Name              string               `json:"name"`
	Description       string               `json:"description"`
	Parameters        []RPCParameter       `json:"parameters"`
	Schema            Schema               `json:"schema"`
	Command           ExecutableCommand    `json:"-"`                // Transport-neutral handle to the original command
	Path              string               `json:"path,omitempty"`   // For REST APIs
	Method            string               `json:"method,omitempty"` // HTTP method
	Tags              []string             `json:"tags,omitempty"`   // OpenAPI tags (auto-derived from the parent command)
	Group             string               `json:"group,omitempty"`  // tool-group this operation belongs to (clicky/tool-group)
	ToolHints         MCPToolHints         `json:"-"`                // MCP annotations and Clicky-specific tool metadata
	DataFunc          DataFunc             `json:"-"`                // Direct data provider, bypasses stdout capture
	ContextDataFunc   ContextDataFunc      `json:"-"`                // Context-aware data provider; preferred over DataFunc when set
	LookupFunc        DataFunc             `json:"-"`                // Direct filter metadata provider
	ContextLookupFunc ContextLookupFunc    `json:"-"`                // Context-aware filter metadata provider; preferred over LookupFunc when set
	Clicky            *ClickyOperationMeta `json:"-"`                // Entity semantics used for OpenAPI extensions
	ResponseType      reflect.Type         `json:"-"`                // Static response type for OpenAPI generation
	ResponseArray     bool                 `json:"-"`                // Response body is an array of ResponseType
	ResponsePaged     bool                 `json:"-"`                // Response body is a paged envelope around ResponseType rows
	ResponseEntityID  bool                 `json:"-"`                // Array item schema should include Clicky _id metadata
}

// ClickySpecMeta is emitted as the OpenAPI-level x-clicky extension.
type ClickySpecMeta struct {
	Surfaces []ClickySurface `json:"surfaces,omitempty"`
}

// ClickySurface describes a UI surface resolved from clicky entity metadata.
type ClickySurface struct {
	Key         string `json:"key"`
	Entity      string `json:"entity"`
	Title       string `json:"title"`
	Parent      string `json:"parent,omitempty"`
	Admin       bool   `json:"admin,omitempty"`
	Description string `json:"description,omitempty"`
	// Icon is an opaque UI icon name (e.g. "database") the frontend resolves to
	// a glyph. Emitted verbatim from the entity's x-clicky-icon annotation.
	Icon string `json:"icon,omitempty"`
	// Path is this surface's position within its Parent group, always separated
	// by PathSeparator (e.g. "jms/incoming/disbursements"). Producers split
	// their own naming convention with SplitPath before emitting it, so the
	// frontend never guesses a delimiter. Empty — the default for every surface
	// that does not opt in — means the group renders as a flat list.
	Path  string `json:"path,omitempty"`
	Order int    `json:"-"`
}

// ClickyOperationMeta is emitted as the OpenAPI operation-level x-clicky
// extension. SurfaceID/Aliases are internal-only fields used while building
// the final surface list.
type ClickyOperationMeta struct {
	SurfaceID          string        `json:"-"`
	Command            string        `json:"command,omitempty"`
	Surface            string        `json:"surface,omitempty"`
	Entity             string        `json:"-"`
	Parent             string        `json:"-"`
	Aliases            []string      `json:"-"`
	Admin              bool          `json:"-"`
	Icon               string        `json:"-"`
	Path               string        `json:"-"`
	Title              string        `json:"-"`
	Verb               string        `json:"verb,omitempty"`
	Scope              string        `json:"scope,omitempty"`
	ActionName         string        `json:"actionName,omitempty"`
	IDParam            string        `json:"idParam,omitempty"`
	SupportsLookup     bool          `json:"supportsLookup,omitempty"`
	SupportsFilterMode bool          `json:"supportsFilterMode,omitempty"`
	Group              string        `json:"group,omitempty"`
	ToolHints          MCPToolHints  `json:"-"`
	OpenAPIToolHints   *MCPToolHints `json:"toolHints,omitempty"`
	Export             *ExportMeta   `json:"export,omitempty"`
	Order              int           `json:"-"`
}

// ExportMeta advertises the representations and scopes an operation can
// download. UI consumers preserve legacy download behavior when this metadata
// is absent.
type ExportMeta struct {
	Formats       []string       `json:"formats,omitempty"`
	Scopes        []string       `json:"scopes,omitempty"`
	AllRowsMode   string         `json:"allRowsMode,omitempty"`
	FormatMaxRows map[string]int `json:"formatMaxRows,omitempty"`
}

// RPCParameter represents a parameter in an RPC operation.
type RPCParameter struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	In          string      `json:"in,omitempty"` // "query", "path", "header" for REST
}

// RPCService represents a collection of RPC operations.
type RPCService struct {
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Operations  []RPCOperation `json:"operations"`
}

// Schema represents a JSON schema for operation input/output.
type Schema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

// Property represents a JSON schema property.
type Property struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Enum        []string    `json:"enum,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

// Config holds configuration for RPC conversion.
type Config struct {
	// DefaultMethod is the default HTTP method for operations.
	DefaultMethod string
	// PathPrefix is the path prefix for REST APIs.
	PathPrefix string
	// AutoGeneratePaths controls whether REST paths are generated from the
	// command hierarchy.
	AutoGeneratePaths bool
	// DefaultTags are applied to all operations.
	DefaultTags []string
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() *Config {
	return &Config{
		DefaultMethod:     "POST",
		PathPrefix:        "/api/v1",
		AutoGeneratePaths: true,
		DefaultTags:       []string{},
	}
}
