package prompt

import (
	"encoding/json"
	"fmt"
	"math"
)

// schemaDoc is the controlled subset of JSON Schema this package generates and
// validates against. It is intentionally small — enough for the select / multi-
// select / text / confirm forms plus caller-supplied object schemas — so the root
// module needs no external validator.
type schemaDoc struct {
	Type        string                `json:"type"`
	Enum        []any                 `json:"enum"`
	Items       *schemaDoc            `json:"items"`
	Properties  map[string]*schemaDoc `json:"properties"`
	Required    []string              `json:"required"`
	UniqueItems bool                  `json:"uniqueItems"`
	MaxItems    *int                  `json:"maxItems"`
}

// Validate reports whether values satisfies schema. A nil/empty schema accepts
// anything (callers that supply no schema opt out of validation).
func Validate(schema json.RawMessage, values map[string]any) error {
	if len(schema) == 0 {
		return nil
	}
	var doc schemaDoc
	if err := json.Unmarshal(schema, &doc); err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}
	if doc.Type != "" && doc.Type != "object" {
		return fmt.Errorf("top-level schema must be an object, got %q", doc.Type)
	}
	for _, key := range doc.Required {
		if _, ok := values[key]; !ok {
			return fmt.Errorf("missing required field %q", key)
		}
	}
	for key, prop := range doc.Properties {
		v, ok := values[key]
		if !ok {
			continue
		}
		if err := validateValue(prop, v); err != nil {
			return fmt.Errorf("field %q: %w", key, err)
		}
	}
	return nil
}

func validateValue(s *schemaDoc, v any) error {
	if s == nil {
		return nil
	}
	switch s.Type {
	case "string":
		sv, ok := v.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", v)
		}
		return validateEnum(s.Enum, sv)
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", v)
		}
	case "number":
		switch v.(type) {
		case float64, float32, int, int64, json.Number:
		default:
			return fmt.Errorf("expected number, got %T", v)
		}
	case "integer":
		// encoding/json decodes request numbers as float64, so reject any value
		// with a fractional part (e.g. 1.5) rather than silently truncating it.
		if err := validateInteger(v); err != nil {
			return err
		}
	case "array":
		arr, ok := v.([]any)
		if !ok {
			return fmt.Errorf("expected array, got %T", v)
		}
		if s.MaxItems != nil && len(arr) > *s.MaxItems {
			return fmt.Errorf("expected at most %d items, got %d", *s.MaxItems, len(arr))
		}
		seen := make(map[string]struct{}, len(arr))
		for i, item := range arr {
			if err := validateValue(s.Items, item); err != nil {
				return fmt.Errorf("item %d: %w", i, err)
			}
			if s.UniqueItems {
				key := fmt.Sprintf("%v", item)
				if _, dup := seen[key]; dup {
					return fmt.Errorf("item %d: duplicate value %v", i, item)
				}
				seen[key] = struct{}{}
			}
		}
	case "object", "":
		// Nested objects / unconstrained values pass; deeper validation is out of
		// scope for the generated forms.
	}
	return nil
}

func validateInteger(v any) error {
	switch n := v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return nil
	case float32:
		if math.Trunc(float64(n)) != float64(n) {
			return fmt.Errorf("expected integer, got %v", v)
		}
	case float64:
		if math.Trunc(n) != n {
			return fmt.Errorf("expected integer, got %v", v)
		}
	case json.Number:
		if _, err := n.Int64(); err != nil {
			return fmt.Errorf("expected integer, got %v", v)
		}
	default:
		return fmt.Errorf("expected integer, got %T", v)
	}
	return nil
}

func validateEnum(enum []any, v string) error {
	if len(enum) == 0 {
		return nil
	}
	for _, e := range enum {
		if s, ok := e.(string); ok && s == v {
			return nil
		}
	}
	return fmt.Errorf("value %q is not one of the allowed options", v)
}
