package prompt

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
)

// schemaDoc is the controlled subset of JSON Schema this package generates and
// validates against. It is intentionally small — enough for the select / multi-
// select / text / confirm forms plus caller-supplied object schemas, including
// fixture forms' allOf if/then/else conditionals — so the root module needs no
// external validator.
type schemaDoc struct {
	Type        string                `json:"type"`
	Enum        []any                 `json:"enum"`
	Const       json.RawMessage       `json:"const"`
	Items       *schemaDoc            `json:"items"`
	Properties  map[string]*schemaDoc `json:"properties"`
	Required    []string              `json:"required"`
	UniqueItems bool                  `json:"uniqueItems"`
	MaxItems    *int                  `json:"maxItems"`
	AllOf       []*schemaDoc          `json:"allOf"`
	If          *schemaDoc            `json:"if"`
	Then        *schemaDoc            `json:"then"`
	Else        *schemaDoc            `json:"else"`
	Not         *schemaDoc            `json:"not"`
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
	return validateObject(&doc, values, "")
}

// validateObject checks values against every schema that applies to each
// property and the union of required keys (base, allOf members and live
// if/then/else branches). Errors name the dotted path of the offending field
// below prefix.
func validateObject(doc *schemaDoc, values map[string]any, prefix string) error {
	props, required, err := effective(doc, values)
	if err != nil {
		return err
	}
	for _, key := range required {
		if _, ok := values[key]; !ok {
			return fmt.Errorf("missing required field %q", fieldPath(prefix, key))
		}
	}
	keys := make([]string, 0, len(props))
	for key := range props {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		v, ok := values[key]
		if !ok {
			continue
		}
		for _, schema := range props[key] {
			if err := validateField(schema, v, fieldPath(prefix, key)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateField(schema *schemaDoc, v any, name string) error {
	if obj, isObj := v.(map[string]any); isObj && schema != nil && (schema.Type == "object" || schema.Type == "") {
		if err := validateObject(schema, obj, name); err != nil {
			return err
		}
		if err := checkConst(schema, v); err != nil {
			return fmt.Errorf("field %q: %w", name, err)
		}
		return nil
	}
	if err := validateValue(schema, v); err != nil {
		return fmt.Errorf("field %q: %w", name, err)
	}
	return nil
}

func fieldPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// effective collects, per property, every schema that constrains it, and the
// union of required keys. A schema applies its own properties and required,
// then the then/else branch its if selects, then each allOf member the same
// way, so every constraint of a combined schema is checked (allOf means all).
func effective(doc *schemaDoc, values map[string]any) (map[string][]*schemaDoc, []string, error) {
	props := map[string][]*schemaDoc{}
	var required []string
	var apply func(branch *schemaDoc, path string) error
	apply = func(branch *schemaDoc, path string) error {
		if branch == nil {
			return nil
		}
		for k, p := range branch.Properties {
			props[k] = append(props[k], p)
		}
		for _, k := range branch.Required {
			if !slices.Contains(required, k) {
				required = append(required, k)
			}
		}
		if branch.If != nil {
			matched, err := matchesIf(branch.If, values)
			if err != nil {
				return fmt.Errorf("invalid schema: %s.if: %w", path, err)
			}
			selected, name := branch.Else, "else"
			if matched {
				selected, name = branch.Then, "then"
			}
			if err := apply(selected, path+"."+name); err != nil {
				return err
			}
		}
		for i, clause := range branch.AllOf {
			member := fmt.Sprintf("%s.allOf[%d]", path, i)
			if clause == nil {
				return fmt.Errorf("invalid schema: %s is null", member)
			}
			if err := apply(clause, member); err != nil {
				return err
			}
		}
		return nil
	}
	if err := apply(doc, "#"); err != nil {
		return nil, nil, err
	}
	return props, required, nil
}

// matchesIf reports whether values satisfies the if schema: every if.required
// key is present and every if.properties predicate holds for the keys that are
// present (an absent key satisfies properties, as in JSON Schema). A predicate
// this package cannot evaluate is an error rather than a guess.
func matchesIf(ifSchema *schemaDoc, values map[string]any) (bool, error) {
	for _, k := range ifSchema.Required {
		if _, ok := values[k]; !ok {
			return false, nil
		}
	}
	keys := make([]string, 0, len(ifSchema.Properties))
	for k := range ifSchema.Properties {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	matched := true
	for _, k := range keys {
		v, present := values[k]
		if !present {
			continue
		}
		holds, err := evaluatePredicate(ifSchema.Properties[k], v)
		if err != nil {
			return false, fmt.Errorf("property %q: %w", k, err)
		}
		matched = matched && holds
	}
	return matched, nil
}

// evaluatePredicate decides a condition sub-schema for a present value from
// const, enum, type, not and nested properties/required. A sub-schema stating
// none of them cannot be decided and is rejected.
func evaluatePredicate(sub *schemaDoc, value any) (bool, error) {
	if sub == nil {
		return false, fmt.Errorf("condition is null")
	}
	decided, holds := false, true
	if sub.Const != nil {
		decided = true
		holds = holds && equalsConst(sub.Const, value)
	}
	if sub.Enum != nil {
		decided = true
		holds = holds && slices.ContainsFunc(sub.Enum, func(e any) bool { return jsonEqual(e, value) })
	}
	if sub.Type != "" {
		decided = true
		matches, err := matchesType(sub.Type, value)
		if err != nil {
			return false, err
		}
		holds = holds && matches
	}
	if sub.Not != nil {
		decided = true
		inner, err := evaluatePredicate(sub.Not, value)
		if err != nil {
			return false, fmt.Errorf("not: %w", err)
		}
		holds = holds && !inner
	}
	if sub.Properties != nil || sub.Required != nil {
		decided = true
		// properties and required constrain objects only.
		if obj, ok := value.(map[string]any); ok {
			matched, err := matchesIf(sub, obj)
			if err != nil {
				return false, err
			}
			holds = holds && matched
		}
	}
	if !decided {
		return false, fmt.Errorf("condition states no const, enum, type, not, properties or required")
	}
	return holds, nil
}

func matchesType(typ string, value any) (bool, error) {
	switch typ {
	case "string":
		_, ok := value.(string)
		return ok, nil
	case "boolean":
		_, ok := value.(bool)
		return ok, nil
	case "number":
		return validateValue(&schemaDoc{Type: "number"}, value) == nil, nil
	case "integer":
		return validateInteger(value) == nil, nil
	case "array":
		_, ok := value.([]any)
		return ok, nil
	case "object":
		_, ok := value.(map[string]any)
		return ok, nil
	case "null":
		return value == nil, nil
	}
	return false, fmt.Errorf("unsupported condition type %q", typ)
}

func checkConst(s *schemaDoc, v any) error {
	if s.Const == nil || equalsConst(s.Const, v) {
		return nil
	}
	return fmt.Errorf("value %v does not equal const %s", v, s.Const)
}

// equalsConst compares v to a const that came from an already-parsed schema,
// so the raw JSON is always valid.
func equalsConst(raw json.RawMessage, v any) bool {
	var want any
	if err := json.Unmarshal(raw, &want); err != nil {
		return false
	}
	return jsonEqual(want, v)
}

// jsonEqual compares values by their JSON shape, so Go ints and decoded
// float64s compare equal. A value that cannot be encoded as JSON can never
// equal a schema literal.
func jsonEqual(a, b any) bool {
	na, okA := toJSONShape(a)
	nb, okB := toJSONShape(b)
	return okA && okB && reflect.DeepEqual(na, nb)
}

func toJSONShape(v any) (any, bool) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, false
	}
	return out, true
}

func validateValue(s *schemaDoc, v any) error {
	if s == nil {
		return nil
	}
	if err := checkConst(s, v); err != nil {
		return err
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
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("expected object, got %T", v)
		}
		return validateObject(s, obj, "")
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
