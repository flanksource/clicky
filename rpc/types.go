package rpc

import "github.com/spf13/cobra"

// DataFunc is a function that returns structured data directly, bypassing stdout capture.
// Used by commands registered via AddCommand to provide data to the HTTP handler.
type DataFunc func(flags map[string]string, args []string) (any, error)

// RPCOperation represents a generic RPC operation that can be converted to various formats
type RPCOperation struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  []RPCParameter `json:"parameters"`
	Schema      Schema         `json:"schema"`
	Command     *cobra.Command `json:"-"`                // Reference to original command
	Path        string         `json:"path,omitempty"`   // For REST APIs
	Method      string         `json:"method,omitempty"` // HTTP method
	Tags        []string       `json:"tags,omitempty"`   // For grouping
	DataFunc    DataFunc       `json:"-"`                // Direct data provider, bypasses stdout capture
}

// RPCParameter represents a parameter in an RPC operation
type RPCParameter struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	In          string      `json:"in,omitempty"` // "query", "path", "header" for REST
}

// RPCService represents a collection of RPC operations
type RPCService struct {
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Operations  []RPCOperation `json:"operations"`
}

// Schema represents a JSON schema for operation input/output
type Schema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

// Property represents a JSON schema property
type Property struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Enum        []string    `json:"enum,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

// Config holds configuration for RPC conversion
type Config struct {
	// Default HTTP method for operations
	DefaultMethod string
	// Path prefix for REST APIs
	PathPrefix string
	// Whether to auto-generate REST paths from command hierarchy
	AutoGeneratePaths bool
	// Tags to apply to all operations
	DefaultTags []string
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultMethod:     "POST",
		PathPrefix:        "/api/v1",
		AutoGeneratePaths: true,
		DefaultTags:       []string{},
	}
}
