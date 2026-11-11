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

// validateObject checks values against the schema's effective properties and
// required keys (base plus live allOf branches). Errors name the dotted path
// of the offending field below prefix.
func validateObject(doc *schemaDoc, values map[string]any, prefix string) error {
	props, required := effective(doc, values)
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
		name := fieldPath(prefix, key)
		if obj, isObj := v.(map[string]any); isObj && props[key] != nil && (props[key].Type == "object" || props[key].Type == "") {
			if err := validateObject(props[key], obj, name); err != nil {
				return err
			}
			if err := checkConst(props[key], v); err != nil {
				return fmt.Errorf("field %q: %w", name, err)
			}
			continue
		}
		if err := validateValue(props[key], v); err != nil {
			return fmt.Errorf("field %q: %w", name, err)
		}
	}
	return nil
}

func fieldPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// effective mirrors schemaform.Effective: an unconditional allOf member always
// contributes its properties and required keys; an if/then/else member
// contributes then when its if matches values, else otherwise. Later members
// win on key collision; required is the union.
func effective(doc *schemaDoc, values map[string]any) (map[string]*schemaDoc, []string) {
	props := make(map[string]*schemaDoc, len(doc.Properties))
	var required []string
	merge := func(branch *schemaDoc) {
		if branch == nil {
			return
		}
		for k, p := range branch.Properties {
			props[k] = p
		}
		for _, k := range branch.Required {
			if !slices.Contains(required, k) {
				required = append(required, k)
			}
		}
	}
	merge(doc)
	for _, clause := range doc.AllOf {
		switch {
		case clause.If == nil && clause.Then == nil:
			merge(clause)
		case matchesIf(clause.If, values):
			merge(clause.Then)
		default:
			merge(clause.Else)
		}
	}
	return props, required
}

// matchesIf mirrors schemaform.matchesIf: every if.required key is present and
// every if.properties predicate holds. A predicate that states nothing
// checkable makes the whole if false, never true.
func matchesIf(ifSchema *schemaDoc, values map[string]any) bool {
	if ifSchema == nil {
		return false
	}
	for _, k := range ifSchema.Required {
		if _, ok := values[k]; !ok {
			return false
		}
	}
	for k, sub := range ifSchema.Properties {
		v, present := values[k]
		if holds, decided := evaluatePredicate(sub, v, present); !decided || !holds {
			return false
		}
	}
	return true
}

// evaluatePredicate mirrors schemaform.evaluatePredicate: const, enum, not and
// nested properties/required decide; anything else (e.g. type alone) is
// undecided so the caller fails closed.
func evaluatePredicate(sub *schemaDoc, value any, present bool) (holds, decided bool) {
	if sub == nil {
		return false, false
	}
	holds = true
	if sub.Const != nil {
		decided = true
		holds = holds && present && equalsConst(sub.Const, value)
	}
	if sub.Enum != nil {
		decided = true
		holds = holds && present && slices.ContainsFunc(sub.Enum, func(e any) bool { return jsonEqual(e, value) })
	}
	if sub.Not != nil {
		inner, innerDecided := evaluatePredicate(sub.Not, value, present)
		if !innerDecided {
			return false, false
		}
		decided = true
		holds = holds && !inner
	}
	if sub.Properties != nil || sub.Required != nil {
		decided = true
		obj, ok := value.(map[string]any)
		holds = holds && ok && matchesIf(sub, obj)
	}
	return holds && decided, decided
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
