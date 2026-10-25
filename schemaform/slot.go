package schemaform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SchemaRequest asks for the schema of an x-clicky-schema property: Source
// is its source, Values the current values of its scope keys.
type SchemaRequest struct {
	Source string
	Values map[string]any
}

// SchemaLoader answers a SchemaRequest with an object schema, or nil (or
// JSON null) when the values select nothing that takes input.
type SchemaLoader func(ctx context.Context, req SchemaRequest) (json.RawMessage, error)

// schemaSlot is an x-clicky-schema property: its fields are built from the
// schema loaded for its scope values, and rebuilt when those change.
type schemaSlot struct {
	b       *builder
	parent  *Schema
	path    []string
	key     string
	spec    SchemaSource
	section string
	// placeholder is the property as declared, restored while nothing loads.
	placeholder *Schema

	signature string
	synced    bool
	fields    []*field
}

func (b *builder) slot(parent *Schema, key string, path []string, section string) (*schemaSlot, error) {
	declared := parent.Properties[key]
	if declared == nil || declared.SchemaSource == nil {
		return nil, fmt.Errorf("%s: declare an x-clicky-schema property in properties, not in a branch", joinPath(path))
	}
	if b.loadSchema == nil {
		return nil, fmt.Errorf("%s: x-clicky-schema %q needs a LoadSchema", joinPath(path), declared.SchemaSource.Source)
	}
	if len(declared.SchemaSource.Scope) == 0 {
		return nil, fmt.Errorf("%s: x-clicky-schema %q has no scope", joinPath(path), declared.SchemaSource.Source)
	}
	return &schemaSlot{
		b: b, parent: parent, path: path, key: key, spec: *declared.SchemaSource,
		section: joinSection(section, declared.Label(key)), placeholder: declared,
	}, nil
}

// scopePath is the path of a scope key: a sibling of the slot.
func (s *schemaSlot) scopePath(name string) []string {
	return appendPath(s.path[:len(s.path)-1], name)
}

// sync loads the schema for the current scope values when they changed, and
// rebuilds the slot's fields; it reports whether it did.
func (s *schemaSlot) sync(ctx context.Context) (bool, error) {
	st := s.b.st
	scope := map[string]any{}
	complete := true
	for _, name := range s.spec.Scope {
		v := st.get(s.scopePath(name))
		scope[name] = v
		complete = complete && !isEmpty(v)
	}
	signature, err := json.Marshal(scope)
	if err != nil {
		return false, fmt.Errorf("%s: scope: %w", joinPath(s.path), err)
	}
	if s.synced && string(signature) == s.signature {
		return false, nil
	}
	first := !s.synced
	s.signature, s.synced = string(signature), true

	var loaded *Schema
	if complete {
		raw, err := s.b.loadSchema(ctx, SchemaRequest{Source: s.spec.Source, Values: scope})
		if err != nil {
			return false, fmt.Errorf("%s: load schema %q: %w", joinPath(s.path), s.spec.Source, err)
		}
		if trimmed := strings.TrimSpace(string(raw)); trimmed != "" && trimmed != "null" {
			if loaded, err = Parse(raw); err != nil {
				return false, fmt.Errorf("%s: schema %q: %w", joinPath(s.path), s.spec.Source, err)
			}
			if !loaded.Type.Has("object") && len(loaded.Properties) == 0 {
				return false, fmt.Errorf("%s: schema %q is not an object schema", joinPath(s.path), s.spec.Source)
			}
		}
	}
	return true, s.install(loaded, first)
}

// install puts loaded (nil: nothing) in place of the property and builds its
// fields. The first install keeps a preset value (a JSON object given on the
// command line); a later one starts from the new schema's defaults.
func (s *schemaSlot) install(loaded *Schema, first bool) error {
	st := s.b.st
	st.unwatch(s.path)
	s.fields = nil
	if loaded == nil {
		s.parent.Properties[s.key] = s.placeholder
		st.replace(s.path, nil)
		return nil
	}
	s.parent.Properties[s.key] = loaded
	current := map[string]any{}
	if first {
		preset, err := presetObject(st.get(s.path))
		if err != nil {
			return fmt.Errorf("%s: %w", joinPath(s.path), err)
		}
		current = preset
	}
	st.replace(s.path, ApplyDefaults(loaded, current))
	steps, err := s.b.build(loaded, s.path, s.section)
	if err != nil {
		return err
	}
	for _, step := range steps {
		f, ok := step.(*field)
		if !ok {
			return fmt.Errorf("%s: schema %q: arrays of objects and nested x-clicky-schema are not supported", joinPath(s.path), s.spec.Source)
		}
		f.slot = s
		s.fields = append(s.fields, f)
	}
	return nil
}

// presetObject reads the value the slot starts from: an object, or the JSON
// text of one (the flag's value).
func presetObject(v any) (map[string]any, error) {
	switch t := v.(type) {
	case nil:
		return map[string]any{}, nil
	case map[string]any:
		return t, nil
	case string:
		if strings.TrimSpace(t) == "" {
			return map[string]any{}, nil
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(t), &out); err != nil {
			return nil, fmt.Errorf("preset %q is not a JSON object: %w", t, err)
		}
		return out, nil
	}
	return nil, fmt.Errorf("preset %v is not an object", v)
}

// syncSlots syncs every slot, reporting whether any rebuilt its fields.
func syncSlots(ctx context.Context, slots []*schemaSlot) (bool, error) {
	changed := false
	for _, s := range slots {
		rebuilt, err := s.sync(ctx)
		if err != nil {
			return false, err
		}
		changed = changed || rebuilt
	}
	return changed, nil
}

// expand lists steps' fields, a slot contributing the fields it has built.
func expand(steps []any) ([]*field, []*schemaSlot) {
	var fields []*field
	var slots []*schemaSlot
	for _, step := range steps {
		switch t := step.(type) {
		case *field:
			fields = append(fields, t)
		case *schemaSlot:
			slots = append(slots, t)
			fields = append(fields, t.fields...)
		}
	}
	return fields, slots
}
