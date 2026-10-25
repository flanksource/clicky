// Package schemaform renders a JSON Schema as an interactive terminal form
// (charmbracelet/huh) with the semantics of clicky-ui's JsonSchemaForm: the same
// widget resolution, `x-*` extensions, `allOf` if/then/else branches,
// `x-discriminator` and `x-on-change`/`x-on-load` listeners. One schema
// therefore drives both the web form and the terminal form.
package schemaform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// Schema mirrors clicky-ui's JsonSchemaProperty: the subset of JSON Schema
// 2020-12 the form understands plus the `x-*` vocabulary it reads.
// Presentation-only keywords (x-layout, x-columns, x-classes, …) have no
// terminal meaning and are ignored.
type Schema struct {
	Type        TypeList `json:"type,omitempty"`
	Title       *string  `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Format      string   `json:"format,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`
	Enum        []any    `json:"enum,omitempty"`
	Const       any      `json:"const,omitempty"`
	Default     any      `json:"default,omitempty"`
	ReadOnly    bool     `json:"readOnly,omitempty"`
	WriteOnly   bool     `json:"writeOnly,omitempty"`

	Minimum    *float64 `json:"minimum,omitempty"`
	Maximum    *float64 `json:"maximum,omitempty"`
	MultipleOf *float64 `json:"multipleOf,omitempty"`
	MinLength  *int     `json:"minLength,omitempty"`
	MaxLength  *int     `json:"maxLength,omitempty"`
	MinItems   *int     `json:"minItems,omitempty"`
	MaxItems   *int     `json:"maxItems,omitempty"`

	UniqueItems       bool               `json:"uniqueItems,omitempty"`
	Items             *Schema            `json:"items,omitempty"`
	Properties        map[string]*Schema `json:"properties,omitempty"`
	Required          []string           `json:"required,omitempty"`
	PatternProperties map[string]*Schema `json:"patternProperties,omitempty"`
	PropertyNames     *Schema            `json:"propertyNames,omitempty"`
	AllOf             []*Schema          `json:"allOf,omitempty"`
	AnyOf             []*Schema          `json:"anyOf,omitempty"`
	OneOf             []*Schema          `json:"oneOf,omitempty"`
	If                *Schema            `json:"if,omitempty"`
	Then              *Schema            `json:"then,omitempty"`
	Else              *Schema            `json:"else,omitempty"`
	Not               *Schema            `json:"not,omitempty"`
	Ref               string             `json:"$ref,omitempty"`

	// AdditionalProperties is the value schema of an open map; AdditionalClosed
	// records an explicit `additionalProperties: false`.
	AdditionalProperties *Schema `json:"-"`
	AdditionalClosed     bool    `json:"-"`

	EnumLabels       map[string]string `json:"x-enum-labels,omitempty"`
	EnumDescriptions map[string]string `json:"x-enum-descriptions,omitempty"`
	EnumDisplay      string            `json:"x-enum-display,omitempty"`
	Hidden           bool              `json:"x-hidden,omitempty"`
	Disabled         bool              `json:"x-disabled,omitempty"`
	Order            []string          `json:"x-order,omitempty"`
	ClickyOrder      *float64          `json:"x-clicky-order,omitempty"`
	Lookup           *LookupSpec       `json:"x-clicky-lookup,omitempty"`
	Clicky           *ParamMeta        `json:"x-clicky,omitempty"`
	Placeholder      string            `json:"x-clicky-placeholder,omitempty"`
	Discriminator    string            `json:"x-discriminator,omitempty"`
	OnChange         []Listener        `json:"x-on-change,omitempty"`
	OnLoad           []Listener        `json:"x-on-load,omitempty"`
	Help             *Help             `json:"x-help,omitempty"`
	ArrayDisplay     string            `json:"x-array-display,omitempty"`
	Item             *ItemSpec         `json:"x-item,omitempty"`
	Unit             string            `json:"x-clicky-unit,omitempty"`
	// SchemaSource marks an object property whose schema is loaded at run time
	// from the values of its Scope (Options.LoadSchema).
	SchemaSource *SchemaSource `json:"x-clicky-schema,omitempty"`

	// order is the document order of Properties; hasConst distinguishes
	// `const: null` from an absent const; raw is the node as authored, used to
	// overlay `$ref` siblings.
	order    []string
	hasConst bool
	raw      json.RawMessage
}

// SchemaSource is the `x-clicky-schema` extension: Source names what to load,
// Scope the sibling keys whose values it depends on.
type SchemaSource struct {
	Source string   `json:"source"`
	Scope  []string `json:"scope"`
}

// ParamMeta is the `x-clicky` parameter extension (e.g. role=filter).
type ParamMeta struct {
	Role string `json:"role,omitempty"`
}

// Help is the `x-help` extension; Body is appended to the description.
type Help struct {
	Body string `json:"body,omitempty"`
}

// ItemSpec is the `x-item` row-summary extension of an array of objects.
type ItemSpec struct {
	Title      []string `json:"title,omitempty"`
	Fallback   string   `json:"fallback,omitempty"`
	Noun       string   `json:"noun,omitempty"`
	NounPlural string   `json:"nounPlural,omitempty"`
}

// LookupSpec is the `x-clicky-lookup` extension: options load from an
// operation's lookup endpoint.
type LookupSpec struct {
	Ref         string         `json:"$ref,omitempty"`
	URL         string         `json:"url,omitempty"`
	Filter      string         `json:"filter"`
	SearchParam string         `json:"searchParam,omitempty"`
	Multi       bool           `json:"multi,omitempty"`
	Scope       map[string]any `json:"scope,omitempty"`
}

// TypeList is a JSON Schema `type`, which may be a single name or a list.
type TypeList []string

func (t *TypeList) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*t = TypeList{single}
		return nil
	}
	var many []string
	if err := json.Unmarshal(data, &many); err != nil {
		return fmt.Errorf("type must be a string or an array of strings: %w", err)
	}
	*t = many
	return nil
}

func (t TypeList) MarshalJSON() ([]byte, error) {
	if len(t) == 1 {
		return json.Marshal(t[0])
	}
	return json.Marshal([]string(t))
}

// Has reports whether name is one of the listed types.
func (t TypeList) Has(name string) bool {
	for _, v := range t {
		if v == name {
			return true
		}
	}
	return false
}

type schemaAlias Schema

// UnmarshalJSON records, beyond the plain keywords, the property document
// order, const presence and the `additionalProperties` bool-or-schema form.
func (s *Schema) UnmarshalJSON(data []byte) error {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("schema must be an object: %w", err)
	}
	if err := json.Unmarshal(data, (*schemaAlias)(s)); err != nil {
		return err
	}
	s.raw = append(json.RawMessage(nil), data...)
	if _, ok := probe["const"]; ok {
		s.hasConst = true
	}
	if props, ok := probe["properties"]; ok {
		order, err := objectKeyOrder(props)
		if err != nil {
			return fmt.Errorf("properties: %w", err)
		}
		s.order = order
	}
	if addl, ok := probe["additionalProperties"]; ok {
		return s.decodeAdditional(addl)
	}
	return nil
}

func (s *Schema) decodeAdditional(raw json.RawMessage) error {
	var closed bool
	if err := json.Unmarshal(raw, &closed); err == nil {
		s.AdditionalProperties, s.AdditionalClosed = nil, !closed
		return nil
	}
	var addl Schema
	if err := json.Unmarshal(raw, &addl); err != nil {
		return fmt.Errorf("additionalProperties: %w", err)
	}
	s.AdditionalProperties, s.AdditionalClosed = &addl, false
	return nil
}

// HasConst reports whether the schema declares `const` (possibly null).
func (s *Schema) HasConst() bool { return s != nil && s.hasConst }

// PropertyOrder returns the property keys in document order.
func (s *Schema) PropertyOrder() []string {
	if s == nil {
		return nil
	}
	if len(s.order) == len(s.Properties) {
		return s.order
	}
	// Properties built in Go carry no document order: keep the recorded keys and
	// append the rest, so every property is still visited exactly once.
	seen := make(map[string]bool, len(s.Properties))
	out := make([]string, 0, len(s.Properties))
	for _, k := range s.order {
		if _, ok := s.Properties[k]; ok && !seen[k] {
			out, seen[k] = append(out, k), true
		}
	}
	for _, k := range sortedKeys(s.Properties) {
		if !seen[k] {
			out = append(out, k)
		}
	}
	return out
}

// Label is the field label: the title, or the key when the title is absent.
// An explicit empty title means the field needs no label of its own.
func (s *Schema) Label(key string) string {
	if s.Title != nil {
		return *s.Title
	}
	return key
}

// Helper is the description joined with the `x-help` body.
func (s *Schema) Helper() string {
	description := strings.TrimSpace(s.Description)
	body := ""
	if s.Help != nil {
		body = strings.TrimSpace(s.Help.Body)
	}
	if description != "" && body != "" && body != description {
		return description + " " + body
	}
	if description != "" {
		return description
	}
	return body
}

// Parse decodes a schema document and resolves its local `$ref` pointers.
func Parse(raw json.RawMessage) (*Schema, error) {
	var root Schema
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}
	r := &refResolver{doc: raw, cache: map[string]*Schema{}, resolving: map[string]bool{}}
	return r.walk(&root)
}

type refResolver struct {
	doc       json.RawMessage
	cache     map[string]*Schema
	resolving map[string]bool
}

func (r *refResolver) walk(s *Schema) (*Schema, error) {
	if s == nil {
		return nil, nil
	}
	if s.Ref != "" {
		return r.resolveRef(s)
	}
	cp := *s
	var err error
	if cp.Items, err = r.walk(cp.Items); err != nil {
		return nil, err
	}
	if cp.AdditionalProperties, err = r.walk(cp.AdditionalProperties); err != nil {
		return nil, err
	}
	if cp.PropertyNames, err = r.walk(cp.PropertyNames); err != nil {
		return nil, err
	}
	for _, slot := range []**Schema{&cp.If, &cp.Then, &cp.Else, &cp.Not} {
		if *slot, err = r.walk(*slot); err != nil {
			return nil, err
		}
	}
	if cp.Properties, err = r.walkMap(cp.Properties); err != nil {
		return nil, err
	}
	if cp.PatternProperties, err = r.walkMap(cp.PatternProperties); err != nil {
		return nil, err
	}
	for _, list := range []*[]*Schema{&cp.AllOf, &cp.AnyOf, &cp.OneOf} {
		if *list, err = r.walkList(*list); err != nil {
			return nil, err
		}
	}
	return &cp, nil
}

func (r *refResolver) walkMap(in map[string]*Schema) (map[string]*Schema, error) {
	if in == nil {
		return nil, nil
	}
	out := make(map[string]*Schema, len(in))
	for k, v := range in {
		walked, err := r.walk(v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		out[k] = walked
	}
	return out, nil
}

func (r *refResolver) walkList(in []*Schema) ([]*Schema, error) {
	if in == nil {
		return nil, nil
	}
	out := make([]*Schema, len(in))
	for i, v := range in {
		walked, err := r.walk(v)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		out[i] = walked
	}
	return out, nil
}

// resolveRef inlines a local `$ref`. Sibling keywords win over the referenced
// body (JSON Schema 2020-12); a cycle ends in a JSON editor stub, as in
// clicky-ui's rehydrateRefs.
func (r *refResolver) resolveRef(s *Schema) (*Schema, error) {
	target, err := r.target(s.Ref)
	if err != nil {
		return nil, err
	}
	siblings, err := withoutKey(s.raw, "$ref")
	if err != nil {
		return nil, err
	}
	if siblings == nil {
		return target, nil
	}
	merged, err := overlay(target, siblings)
	if err != nil {
		return nil, fmt.Errorf("$ref %s siblings: %w", s.Ref, err)
	}
	return r.walk(merged)
}

// overlay returns a copy of base whose keywords present in raw are replaced
// wholesale by raw's — the `{...base, ...raw}` spread clicky-ui uses for
// `$ref` siblings and listener patches. base is never mutated.
func overlay(base *Schema, raw json.RawMessage) (*Schema, error) {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil, err
	}
	var top Schema
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, err
	}
	out := *base
	dst, src := reflect.ValueOf(&out).Elem(), reflect.ValueOf(&top).Elem()
	for i := 0; i < dst.NumField(); i++ {
		name, _, _ := strings.Cut(dst.Type().Field(i).Tag.Get("json"), ",")
		if _, ok := keys[name]; ok && name != "" && name != "-" {
			dst.Field(i).Set(src.Field(i))
		}
	}
	if _, ok := keys["const"]; ok {
		out.Const, out.hasConst = top.Const, true
	}
	if _, ok := keys["properties"]; ok {
		out.order = top.order
	}
	if _, ok := keys["additionalProperties"]; ok {
		out.AdditionalProperties, out.AdditionalClosed = top.AdditionalProperties, top.AdditionalClosed
	}
	return &out, nil
}

func (r *refResolver) target(ref string) (*Schema, error) {
	if cached, ok := r.cache[ref]; ok {
		return cached, nil
	}
	if r.resolving[ref] {
		return &Schema{Format: "json", Description: "↻ recursive schema"}, nil
	}
	raw, err := resolvePointer(r.doc, ref)
	if err != nil {
		return nil, err
	}
	var target Schema
	if err := json.Unmarshal(raw, &target); err != nil {
		return nil, fmt.Errorf("$ref %s: %w", ref, err)
	}
	r.resolving[ref] = true
	walked, err := r.walk(&target)
	delete(r.resolving, ref)
	if err != nil {
		return nil, fmt.Errorf("$ref %s: %w", ref, err)
	}
	r.cache[ref] = walked
	return walked, nil
}

// resolvePointer follows a local RFC 6901 fragment pointer (`#/$defs/x`)
// through the raw document. Remote refs are not supported and fail loudly.
func resolvePointer(doc json.RawMessage, ref string) (json.RawMessage, error) {
	if !strings.HasPrefix(ref, "#") {
		return nil, fmt.Errorf("$ref %q: only local fragment pointers are supported", ref)
	}
	fragment, err := url.PathUnescape(ref[1:])
	if err != nil {
		return nil, fmt.Errorf("$ref %q: %w", ref, err)
	}
	if fragment == "" {
		return doc, nil
	}
	if !strings.HasPrefix(fragment, "/") {
		return nil, fmt.Errorf("$ref %q: not a JSON pointer", ref)
	}
	current := doc
	for _, token := range strings.Split(fragment[1:], "/") {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		next, err := pointerStep(current, token)
		if err != nil {
			return nil, fmt.Errorf("$ref %q: %w", ref, err)
		}
		current = next
	}
	return current, nil
}

func pointerStep(current json.RawMessage, token string) (json.RawMessage, error) {
	if trimmed := bytes.TrimSpace(current); len(trimmed) > 0 && trimmed[0] == '[' {
		var list []json.RawMessage
		if err := json.Unmarshal(current, &list); err != nil {
			return nil, err
		}
		index, err := strconv.Atoi(token)
		if err != nil || index < 0 || index >= len(list) {
			return nil, fmt.Errorf("index %q out of range", token)
		}
		return list[index], nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(current, &object); err != nil {
		return nil, fmt.Errorf("segment %q: %w", token, err)
	}
	next, ok := object[token]
	if !ok {
		return nil, fmt.Errorf("segment %q not found", token)
	}
	return next, nil
}

// withoutKey returns raw with key removed, or nil when nothing else remains.
func withoutKey(raw json.RawMessage, key string) (json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	delete(object, key)
	if len(object) == 0 {
		return nil, nil
	}
	return json.Marshal(object)
}

// objectKeyOrder returns the keys of a raw JSON object in document order.
func objectKeyOrder(raw json.RawMessage) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("expected an object")
	}
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		keys = append(keys, tok.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
	}
	return keys, nil
}
