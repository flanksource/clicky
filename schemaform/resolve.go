package schemaform

// Ported from clicky-ui json-schema-form-resolve.ts / -union-model.ts.

// Kind is the control a property renders as.
type Kind string

const (
	KindString     Kind = "string"
	KindLookup     Kind = "lookup"
	KindJSON       Kind = "json"
	KindUnion      Kind = "union"
	KindEnum       Kind = "enum"
	KindDate       Kind = "date"
	KindTextarea   Kind = "textarea"
	KindMarkdown   Kind = "markdown"
	KindBoolean    Kind = "boolean"
	KindNumber     Kind = "number"
	KindTags       Kind = "tags"        // array of scalars: one tag list
	KindMultiEnum  Kind = "multi-enum"  // array of enum items: a list of choices
	KindObjectList Kind = "object-list" // array of objects: per-item sub-forms
	KindStringMap  Kind = "string-map"
	KindObject     Kind = "object"
)

// Option is one choice of an enum or lookup control. Value is the option's
// string form; Raw is the typed value written to the answer.
type Option struct {
	Value string
	Label string
	Raw   any
	// Description explains the option (x-enum-descriptions, an item's
	// description); the form shows it for the highlighted and the chosen option.
	Description string
}

// Control is a property resolved to a render-ready control.
type Control struct {
	Kind    Kind
	Options []Option
	// Branch is the native schema a value-or-template union renders as.
	Branch *Schema
}

// ResolveControl infers the control from a property schema's keywords (never
// its name), first match wins — the same precedence as clicky-ui's
// resolveControl.
func ResolveControl(prop *Schema) Control {
	if prop.Lookup != nil && prop.Lookup.Filter != "" {
		return Control{Kind: KindLookup}
	}
	if prop.Format == "json" {
		return Control{Kind: KindJSON}
	}
	if len(unionBranches(prop)) > 1 {
		return Control{Kind: KindUnion}
	}
	if len(prop.Enum) > 0 {
		return Control{Kind: KindEnum, Options: enumOptions(prop)}
	}
	if branch := enumBranch(prop); branch != nil {
		return Control{Kind: KindEnum, Options: enumOptions(branch)}
	}
	if native := templateNativeBranch(prop); native != nil {
		control := ResolveControl(native)
		control.Branch = native
		return control
	}
	switch {
	case prop.Format == "date" || prop.Format == "date-time":
		return Control{Kind: KindDate}
	case prop.Format == "textarea" && prop.Type.Has("string"):
		return Control{Kind: KindTextarea}
	case prop.Format == "md" && prop.Type.Has("string"):
		return Control{Kind: KindMarkdown}
	case prop.Type.Has("boolean"):
		return Control{Kind: KindBoolean}
	case prop.Type.Has("integer") || prop.Type.Has("number"):
		return Control{Kind: KindNumber}
	case prop.Type.Has("array"):
		return resolveArray(prop)
	case isOpenStringMap(prop):
		return Control{Kind: KindStringMap}
	case rendersAsObject(prop) && (len(prop.Properties) > 0 || prop.Properties != nil || len(prop.AllOf) > 0):
		return Control{Kind: KindObject}
	case rendersAsObject(prop):
		return Control{Kind: KindStringMap}
	}
	return Control{Kind: KindString}
}

func resolveArray(prop *Schema) Control {
	items := prop.Items
	switch {
	case items != nil && len(items.Enum) > 0:
		return Control{Kind: KindMultiEnum, Options: enumOptions(items)}
	case scalarItemsType(items) != "":
		return Control{Kind: KindTags}
	case items != nil && rendersAsObject(items):
		return Control{Kind: KindObjectList}
	}
	return Control{Kind: KindJSON}
}

// enumOptions labels each enum value "Label (value)" from x-enum-labels and
// describes it from x-enum-descriptions.
func enumOptions(prop *Schema) []Option {
	out := make([]Option, 0, len(prop.Enum))
	for _, v := range prop.Enum {
		value := formatScalar(v)
		label := value
		if l := prop.EnumLabels[value]; l != "" && l != value {
			label = l + " (" + value + ")"
		}
		out = append(out, Option{Value: value, Label: label, Raw: normalize(v), Description: prop.EnumDescriptions[value]})
	}
	return out
}

// enumBranch combines literal choices across anyOf/oneOf branches, including
// enum-or-template unions.
func enumBranch(prop *Schema) *Schema {
	branches := append(append([]*Schema(nil), prop.AnyOf...), prop.OneOf...)
	literalsOnly := true
	for _, b := range branches {
		if len(b.Enum) == 0 && !isStringConst(b) {
			literalsOnly = false
		}
	}
	var choices []*Schema
	for _, b := range branches {
		if len(b.Enum) > 0 || (literalsOnly && isStringConst(b)) {
			choices = append(choices, b)
		}
	}
	if len(choices) == 0 {
		return nil
	}
	merged := *choices[0]
	merged.Enum, merged.EnumLabels = nil, map[string]string{}
	for _, c := range choices {
		values := c.Enum
		if len(values) == 0 {
			values = []any{c.Const}
		}
		for _, v := range values {
			if !containsJSON(merged.Enum, v) {
				merged.Enum = append(merged.Enum, v)
			}
		}
		for k, v := range c.EnumLabels {
			merged.EnumLabels[k] = v
		}
	}
	return &merged
}

func isStringConst(s *Schema) bool {
	_, ok := s.Const.(string)
	return s.HasConst() && ok
}

// templateNativeBranch handles a two-member anyOf of a template string (with a
// pattern) and a native number/integer/boolean/date branch: it renders as the
// native branch's control.
func templateNativeBranch(prop *Schema) *Schema {
	if len(prop.AnyOf) != 2 {
		return nil
	}
	var template, native *Schema
	for _, part := range prop.AnyOf {
		if template == nil && part.Type.Has("string") && part.Pattern != "" {
			template = part
		} else {
			native = part
		}
	}
	if template == nil || native == nil {
		return nil
	}
	if !native.Type.Has("number") && !native.Type.Has("integer") && !native.Type.Has("boolean") &&
		native.Format != "date" && native.Format != "date-time" {
		return nil
	}
	merged := *prop
	merged.AnyOf = nil
	merged.Type, merged.Format = native.Type, native.Format
	if native.Minimum != nil {
		merged.Minimum = native.Minimum
	}
	if native.Maximum != nil {
		merged.Maximum = native.Maximum
	}
	return &merged
}

// unionBranches returns the branches of a genuine union (mixed types, or
// object/array members); value-or-template unions are not unions.
func unionBranches(prop *Schema) []*Schema {
	branches := prop.OneOf
	if branches == nil {
		branches = prop.AnyOf
	}
	if len(branches) > 1 {
		types := map[string]bool{}
		template := false
		for _, b := range branches {
			types[branchType(b)] = true
			if b.Type.Has("string") && b.Pattern != "" {
				template = true
			}
		}
		template = template && len(branches) == 2
		if !template && (len(types) > 1 || types["object"] || types["array"]) {
			return branches
		}
	}
	if len(prop.Type) > 1 {
		out := make([]*Schema, len(prop.Type))
		for i, t := range prop.Type {
			out[i] = &Schema{Type: TypeList{t}}
		}
		return out
	}
	return nil
}

func branchType(s *Schema) string {
	if s.HasConst() {
		switch s.Const.(type) {
		case nil:
			return "null"
		case string:
			return "string"
		case bool:
			return "boolean"
		case float64:
			return "number"
		}
		return "object"
	}
	if s.Properties != nil || s.AdditionalProperties != nil {
		return "object"
	}
	if len(s.Type) == 0 {
		return "value"
	}
	return s.Type[0]
}

// matchesUnionBranch reports whether value fits a union branch.
func matchesUnionBranch(s *Schema, value any) bool {
	if s.HasConst() {
		return jsonEqual(s.Const, value)
	}
	if s.Enum != nil {
		return containsJSON(s.Enum, value)
	}
	switch {
	case s.Type.Has("null"):
		return value == nil
	case s.Type.Has("array"):
		_, ok := value.([]any)
		return ok
	case s.Type.Has("object") || s.Properties != nil || s.AdditionalProperties != nil:
		object, ok := asObject(value)
		if !ok {
			return false
		}
		for k, prop := range s.Properties {
			if prop.HasConst() && !jsonEqual(object[k], prop.Const) {
				return false
			}
		}
		return true
	case s.Type.Has("integer"):
		n, ok := value.(float64)
		return ok && n == float64(int64(n))
	case s.Type.Has("number"):
		_, ok := value.(float64)
		return ok
	case s.Type.Has("boolean"):
		_, ok := value.(bool)
		return ok
	case s.Type.Has("string"):
		_, ok := value.(string)
		return ok
	}
	return false
}

// rendersAsObject accepts schemas that omit `type` but use object-only
// keywords; an explicit incompatible type always wins.
func rendersAsObject(prop *Schema) bool {
	if prop.Type.Has("object") {
		return true
	}
	if len(prop.Type) > 0 {
		return false
	}
	if prop.Properties != nil || prop.Required != nil || prop.AdditionalProperties != nil ||
		prop.AdditionalClosed || prop.PatternProperties != nil || prop.PropertyNames != nil {
		return true
	}
	for _, clause := range prop.AllOf {
		if !isConditional(clause) && rendersAsObject(clause) {
			return true
		}
	}
	return false
}

// isOpenStringMap reports an object whose entries are described by an
// additionalProperties schema or patternProperties: a key/value control.
func isOpenStringMap(prop *Schema) bool {
	if !rendersAsObject(prop) {
		return false
	}
	return prop.AdditionalProperties != nil || len(prop.PatternProperties) > 0
}

// scalarItemsType returns the scalar type an array's items hold — the case the
// array renders as ONE tag list — or "" when the items need richer controls.
func scalarItemsType(items *Schema) string {
	if items == nil {
		return "string"
	}
	if items.Enum != nil || items.HasConst() || items.Properties != nil || items.Items != nil ||
		items.AdditionalProperties != nil || items.AllOf != nil || items.OneOf != nil || items.AnyOf != nil {
		return ""
	}
	switch {
	case len(items.Type) == 0 || items.Type.Has("string"):
		return "string"
	case items.Type.Has("integer"):
		return "integer"
	case items.Type.Has("number"):
		return "number"
	}
	return ""
}
