package entity

import (
	"fmt"

	"github.com/flanksource/clicky/api"
)

// Source kinds for the declarative FilterSourceSpec.
const (
	SourceStatic = "static"
	SourceEntity = "entity"
	SourceFunc   = "func" // Go-only; not constructible from a spec.
	SourceCustom = "custom"
)

// FilterSpec is the JSON/YAML-authorable form of a NamedFilter. It is also the
// shape emitted for OpenAPI (NamedFilter.Spec), so a Go-defined filter and a
// declaratively-defined one are indistinguishable to clients.
type FilterSpec struct {
	Name   string           `json:"name" yaml:"name"`
	Label  string           `json:"label,omitempty" yaml:"label,omitempty"`
	Type   string           `json:"type,omitempty" yaml:"type,omitempty"`
	Multi  bool             `json:"multi,omitempty" yaml:"multi,omitempty"`
	Source FilterSourceSpec `json:"source" yaml:"source"`
}

// FilterSourceSpec is the declarative description of where a filter's options
// come from. "static" and "entity" are constructible from a spec; "func" is
// Go-only and only ever appears on the emit side (NamedFilter.Spec).
type FilterSourceSpec struct {
	Kind string `json:"kind" yaml:"kind"`
	// Options is the value→label map for a static source.
	Options map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
	// Entity is the source entity name for an entity source.
	Entity string `json:"entity,omitempty" yaml:"entity,omitempty"`
}

// FilterFromSpec builds (but does not register) a NamedFilter from a declarative
// spec, validating the source kind.
func FilterFromSpec(s FilterSpec) (NamedFilter, error) {
	if s.Name == "" {
		return NamedFilter{}, fmt.Errorf("filter spec: name must not be empty")
	}

	var source FilterSource
	switch s.Source.Kind {
	case SourceStatic:
		options := make(map[string]api.Textable, len(s.Source.Options))
		for value, label := range s.Source.Options {
			options[value] = api.Text{Content: label}
		}
		source = StaticOptions(options)
	case SourceEntity:
		if s.Source.Entity == "" {
			return NamedFilter{}, fmt.Errorf("filter spec %q: entity source requires an entity name", s.Name)
		}
		source = EntityOptions(s.Source.Entity)
	case "":
		return NamedFilter{}, fmt.Errorf("filter spec %q: source.kind is required", s.Name)
	default:
		return NamedFilter{}, fmt.Errorf("filter spec %q: source kind %q is not constructible from a spec", s.Name, s.Source.Kind)
	}

	return NamedFilter{
		Name:   s.Name,
		Label:  s.Label,
		Type:   s.Type,
		Multi:  s.Multi,
		Source: source,
	}, nil
}

// RegisterFilterSpec registers a declaratively-defined named filter. Like
// RegisterFilter it is a setup-time call and panics on an invalid spec.
func RegisterFilterSpec(s FilterSpec) {
	f, err := FilterFromSpec(s)
	if err != nil {
		panic("entity.RegisterFilterSpec: " + err.Error())
	}
	RegisterFilter(f)
}

// Spec emits the declarative form of a NamedFilter, used for OpenAPI generation
// and round-tripping. A func/custom source emits its kind without detail (its
// options are resolved server-side via the lookup endpoint).
func (f NamedFilter) Spec() FilterSpec {
	return FilterSpec{
		Name:   f.Name,
		Label:  f.Label,
		Type:   f.controlType(),
		Multi:  f.Multi,
		Source: sourceSpec(f.Source),
	}
}

// sourceSpec describes a FilterSource declaratively. Built-in sources emit full
// detail; an external source emits only the "custom" kind (its options resolve
// server-side via the lookup endpoint).
func sourceSpec(src FilterSource) FilterSourceSpec {
	switch s := src.(type) {
	case staticSource:
		options := make(map[string]string, len(s.options))
		for value, label := range s.options {
			options[value] = labelString(label)
		}
		return FilterSourceSpec{Kind: SourceStatic, Options: options}
	case entitySource:
		return FilterSourceSpec{Kind: SourceEntity, Entity: s.entityName}
	case funcSource:
		return FilterSourceSpec{Kind: SourceFunc}
	default:
		return FilterSourceSpec{Kind: SourceCustom}
	}
}

func labelString(label api.Textable) string {
	if label == nil {
		return ""
	}
	return label.String()
}
