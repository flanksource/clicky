package rpc

import (
	"reflect"

	"github.com/spf13/cobra"
)

// DataFunc is a function that returns structured data directly, bypassing stdout capture.
// Used by commands registered via AddCommand to provide data to the HTTP handler.
type DataFunc func(flags map[string]string, args []string) (any, error)

// RPCOperation represents a generic RPC operation that can be converted to various formats
type RPCOperation struct {
	Name             string               `json:"name"`
	Description      string               `json:"description"`
	Parameters       []RPCParameter       `json:"parameters"`
	Schema           Schema               `json:"schema"`
	Command          *cobra.Command       `json:"-"`                // Reference to original command
	Path             string               `json:"path,omitempty"`   // For REST APIs
	Method           string               `json:"method,omitempty"` // HTTP method
	Tags             []string             `json:"tags,omitempty"`   // For grouping
	DataFunc         DataFunc             `json:"-"`                // Direct data provider, bypasses stdout capture
	LookupFunc       DataFunc             `json:"-"`                // Direct filter metadata provider
	Clicky           *ClickyOperationMeta `json:"-"`                // Entity semantics used for OpenAPI extensions
	ResponseType     reflect.Type         `json:"-"`                // Static response type for OpenAPI generation
	ResponseArray    bool                 `json:"-"`                // Response body is an array of ResponseType
	ResponseEntityID bool                 `json:"-"`                // Array item schema should include Clicky _id metadata
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
	Order       int    `json:"-"`
}

// ClickyOperationMeta is emitted as the OpenAPI operation-level x-clicky
// extension. SurfaceID/Aliases are internal-only fields used while building
// the final surface list.
type ClickyOperationMeta struct {
	SurfaceID          string   `json:"-"`
	Command            string   `json:"command,omitempty"`
	Surface            string   `json:"surface,omitempty"`
	Entity             string   `json:"-"`
	Parent             string   `json:"-"`
	Aliases            []string `json:"-"`
	Admin              bool     `json:"-"`
	Verb               string   `json:"verb,omitempty"`
	Scope              string   `json:"scope,omitempty"`
	ActionName         string   `json:"actionName,omitempty"`
	IDParam            string   `json:"idParam,omitempty"`
	SupportsLookup     bool     `json:"supportsLookup,omitempty"`
	SupportsFilterMode bool     `json:"supportsFilterMode,omitempty"`
	Order              int      `json:"-"`
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
