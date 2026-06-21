package entity

import (
	"context"

	"github.com/flanksource/clicky/api"
)

// FilterContext carries the request-scoped state a FilterSource needs to resolve
// its options. It is the single representation both static and dynamic entities
// can produce: a static entity reflects its typed ListOpts into Params, a dynamic
// entity already holds the flag map.
type FilterContext struct {
	// Context is the request context (HTTP request or CLI command). It may be
	// nil when a source is invoked outside a request (e.g. shell completion).
	Context context.Context
	// Key is the bound flag/field name this filter resolves on the current
	// entity (e.g. "owner" when a "users" filter is attached via .As("owner")).
	Key string
	// Params holds the current values of every flag/filter on the entity, so a
	// source can narrow its options against sibling selections (cascading).
	Params map[string]string
}

// Ctx returns the context, defaulting to context.Background when none was set.
func (fc FilterContext) Ctx() context.Context {
	if fc.Context == nil {
		return context.Background()
	}
	return fc.Context
}

// FilterSource produces the options for a NamedFilter. Implementations are
// type-agnostic — they read FilterContext.Params rather than a typed struct — so
// one source serves any entity that references the filter.
type FilterSource interface {
	// Options returns the available options, optionally narrowed by query, and
	// the true total count behind the (possibly capped) returned set. An empty
	// query means "the head set". A non-positive limit means "no cap".
	Options(fc FilterContext, query string, limit int) (options map[string]api.Textable, total int, err error)
	// Resolve labels the currently-selected raw value(s) for display. A value
	// with no known label is echoed back as plain text.
	Resolve(fc FilterContext, values []string) (map[string]api.Textable, error)
}

// NamedFilter is a reusable filter definition registered under a unique name and
// attached to entities by reference. Authored in Go (RegisterFilter) or
// declaratively (RegisterFilterSpec), both normalize to this type.
type NamedFilter struct {
	// Name is the unique registry key.
	Name string
	// Label is the human-facing control label. Defaults to Name when empty.
	Label string
	// Type is the UI control type: "select" (default), "multi-select", "date",
	// "from", or "to".
	Type string
	// Multi reports whether the control accepts multiple selections.
	Multi bool
	// Source resolves the filter's options.
	Source FilterSource
}

// label returns the display label, defaulting to the name.
func (f NamedFilter) label() string {
	if f.Label != "" {
		return f.Label
	}
	return f.Name
}

// controlType returns the UI control type, defaulting to "select".
func (f NamedFilter) controlType() string {
	if f.Type != "" {
		return f.Type
	}
	if f.Multi {
		return "multi-select"
	}
	return "select"
}
