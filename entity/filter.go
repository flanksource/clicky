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

// FilterOptions is one answer from a FilterSource: the options it offers, how
// many rows hold each of them, and how many options exist behind the set.
type FilterOptions struct {
	// Options maps each option value to its display label.
	Options map[string]api.Textable
	// Counts maps an option value to the number of rows holding it, keyed like
	// Options. It is nil when the source does not count; a value it has no count
	// for is absent rather than zero.
	Counts map[string]int
	// Total is the true number of options behind the (possibly capped) Options.
	Total int
}

// FilterSource produces the options for a NamedFilter. Implementations are
// type-agnostic — they read FilterContext.Params rather than a typed struct — so
// one source serves any entity that references the filter.
type FilterSource interface {
	// Options returns the available options, optionally narrowed by query, and
	// the true total count behind the (possibly capped) returned set. An empty
	// query means "the head set". A non-positive limit means "no cap".
	//
	// Deprecated: Use CountedFilterSource.CountedOptions for count-aware lookup results.
	Options(fc FilterContext, query string, limit int) (options map[string]api.Textable, total int, err error)
	// Resolve labels the currently-selected raw value(s) for display. A value
	// with no known label is echoed back as plain text.
	Resolve(fc FilterContext, values []string) (map[string]api.Textable, error)
}

// CountedFilterSource extends FilterSource with per-value row counts. Existing
// FilterSource implementations remain valid; lookup resolution uses this richer
// answer only when the source implements it.
type CountedFilterSource interface {
	FilterSource
	CountedOptions(fc FilterContext, query string, limit int) (FilterOptions, error)
}

func filterSourceOptions(source FilterSource, fc FilterContext, query string, limit int) (FilterOptions, error) {
	if counted, ok := source.(CountedFilterSource); ok {
		return counted.CountedOptions(fc, query, limit)
	}
	options, total, err := source.Options(fc, query, limit)
	if err != nil {
		return FilterOptions{}, err
	}
	return FilterOptions{Options: options, Total: total}, nil
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
	// Limit caps the option set this filter enumerates in one shot. Zero takes
	// MaxLookupOptions, which is also the ceiling. Declare a smaller one for a
	// field whose cardinality makes a full head set useless to pick from: what
	// is past it is reached by typing, not by scrolling.
	Limit int
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
