package rpc

import (
	"reflect"
	"strconv"
	"strings"
	"time"
)

var timeType = reflect.TypeOf(time.Time{})

// SchemaDescriber lets a type emit its own JSON-schema fragment instead of being
// structurally reflected. Known keys (type/title/format/description) populate the
// matching OpenAPISchema fields; everything else (e.g. x-clicky-*) goes into
// Extensions. Used by types whose string form hides a richer struct (e.g.
// commons-db's types.EnvVar, which renders as a plain secret-ref string).
type SchemaDescriber interface {
	JSONSchema() map[string]any
}

var schemaDescriberType = reflect.TypeOf((*SchemaDescriber)(nil)).Elem()

// SchemaForStruct reflects v's type into an OpenAPISchema, honoring the
// clicky:"..." field tags and the SchemaDescriber interface. It is the public
// entry point for schema generators that build forms from Go structs.
func SchemaForStruct(v any) *OpenAPISchema {
	return NewOpenAPIGenerator(nil).convertGoTypeToOpenAPI(reflect.TypeOf(v))
}

// convertGoTypeToOpenAPI is the entry point: starts a fresh visited set so each
// top-level conversion can revisit shared types without confusing siblings.
func (g *OpenAPIGenerator) convertGoTypeToOpenAPI(t reflect.Type) *OpenAPISchema {
	return g.convertGoTypeWithSeen(t, map[reflect.Type]struct{}{})
}

func (g *OpenAPIGenerator) convertGoTypeWithSeen(t reflect.Type, seen map[reflect.Type]struct{}) *OpenAPISchema {
	if t == nil {
		return &OpenAPISchema{Type: "object"}
	}

	for t.Kind() == reflect.Ptr {
		schema := g.convertGoTypeWithSeen(t.Elem(), seen)
		schema.Nullable = true
		return schema
	}

	if t == timeType {
		return &OpenAPISchema{Type: "string", Format: "date-time"}
	}

	if t.Implements(schemaDescriberType) || reflect.PtrTo(t).Implements(schemaDescriberType) {
		describer := reflect.New(t).Elem().Interface().(SchemaDescriber)
		return schemaFromMap(describer.JSONSchema())
	}

	switch t.Kind() {
	case reflect.Bool:
		return &OpenAPISchema{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return &OpenAPISchema{Type: "integer", Format: "int32"}
	case reflect.Int64:
		return &OpenAPISchema{Type: "integer", Format: "int64"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return &OpenAPISchema{Type: "integer", Format: "int32"}
	case reflect.Uint64, reflect.Uintptr:
		return &OpenAPISchema{Type: "integer", Format: "int64"}
	case reflect.Float32:
		return &OpenAPISchema{Type: "number", Format: "float"}
	case reflect.Float64:
		return &OpenAPISchema{Type: "number", Format: "double"}
	case reflect.String:
		return &OpenAPISchema{Type: "string"}
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return &OpenAPISchema{Type: "string", Format: "byte"}
		}
		return &OpenAPISchema{
			Type:  "array",
			Items: g.convertGoTypeWithSeen(t.Elem(), seen),
		}
	case reflect.Map:
		return &OpenAPISchema{
			Type:                 "object",
			AdditionalProperties: g.convertGoTypeWithSeen(t.Elem(), seen),
		}
	case reflect.Struct:
		return g.convertStructTypeWithSeen(t, seen)
	case reflect.Interface:
		return &OpenAPISchema{
			Type:        "object",
			Description: "Dynamic response value",
		}
	default:
		return &OpenAPISchema{
			Type:        "object",
			Description: "Unsupported Go response type: " + t.String(),
		}
	}
}

func (g *OpenAPIGenerator) convertStructTypeWithSeen(t reflect.Type, seen map[reflect.Type]struct{}) *OpenAPISchema {
	if _, ok := seen[t]; ok {
		// Self-referencing or mutually-recursive struct (e.g. Policy.Activities []Activity, Activity.Policy Policy).
		// Return an opaque object schema to break the cycle.
		return &OpenAPISchema{
			Type:        "object",
			Description: "Recursive reference to " + t.String(),
		}
	}
	seen[t] = struct{}{}
	defer delete(seen, t)

	schema := &OpenAPISchema{
		Type:       "object",
		Properties: map[string]*OpenAPISchema{},
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		if isHiddenResponseField(field) {
			continue
		}

		name, omitempty := responseJSONName(field)
		fieldType := field.Type
		for fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		if field.Anonymous && name == "" && fieldType.Kind() == reflect.Struct && fieldType != timeType {
			embedded := g.convertStructTypeWithSeen(fieldType, seen)
			for key, value := range embedded.Properties {
				schema.Properties[key] = value
			}
			schema.Required = append(schema.Required, embedded.Required...)
			continue
		}
		if name == "" {
			name = field.Name
		}

		sub := g.convertGoTypeWithSeen(field.Type, seen)
		ct := parseClickyTag(field)
		applyClickyTag(sub, ct)
		schema.Properties[name] = sub
		// A clicky-tagged field is a form field: its requiredness is explicit
		// (the `required` token). Untagged fields keep the response-schema
		// heuristic (required unless json omitempty / an inherently optional kind).
		required := ct.Required
		if field.Tag.Get("clicky") == "" {
			required = !omitempty && !isOptionalResponseField(field.Type)
		}
		if required {
			schema.Required = append(schema.Required, name)
		}
	}

	if len(schema.Properties) == 0 {
		schema.Properties = nil
	}
	if len(schema.Required) == 0 {
		schema.Required = nil
	}
	return schema
}

func isHiddenResponseField(field reflect.StructField) bool {
	if field.Tag.Get("json") == "-" {
		return true
	}
	for _, part := range strings.Split(field.Tag.Get("pretty"), ",") {
		if strings.TrimSpace(part) == "hide" {
			return true
		}
	}
	return false
}

func responseJSONName(field reflect.StructField) (string, bool) {
	tag := field.Tag.Get("json")
	if tag == "" {
		return "", false
	}
	name, opts, _ := strings.Cut(tag, ",")
	if name == "-" {
		return "-", true
	}
	return name, strings.Contains(","+opts+",", ",omitempty,")
}

func isOptionalResponseField(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface:
		return true
	default:
		return false
	}
}

// clickyTag is the parsed form of a field's clicky:"..." struct tag, the
// vocabulary that maps Go fields onto form widgets.
type clickyTag struct {
	Component     string // type=<component>     -> x-clicky-component
	Title         string // title=<label>        -> title
	Description   string // desc=<text>           -> description
	Format        string // format=<fmt>          -> format (e.g. password)
	DefaultSource string // source=<value|secret> -> x-clicky-default-source
	Property      string // property=<key>        -> x-clicky-property (consumer-defined nesting)
	Required      bool   // required              -> include in required[]
	Order         int    // order=<n>             -> x-clicky-order (per-property render order)
	HasOrder      bool   // whether order= was set
}

// parseClickyTag reads the comma-separated clicky:"..." tag tokens.
func parseClickyTag(field reflect.StructField) clickyTag {
	var c clickyTag
	for _, part := range strings.Split(field.Tag.Get("clicky"), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, val, hasVal := strings.Cut(part, "=")
		switch {
		case part == "required":
			c.Required = true
		case key == "type" && hasVal:
			c.Component = val
		case key == "title" && hasVal:
			c.Title = val
		case key == "desc" && hasVal:
			c.Description = val
		case key == "format" && hasVal:
			c.Format = val
		case key == "source" && hasVal:
			c.DefaultSource = val
		case key == "property" && hasVal:
			c.Property = val
		case key == "order" && hasVal:
			if n, err := strconv.Atoi(val); err == nil {
				c.Order = n
				c.HasOrder = true
			}
		}
	}
	return c
}

// applyClickyTag overlays a parsed clicky tag onto a field's sub-schema.
func applyClickyTag(s *OpenAPISchema, c clickyTag) {
	if c.Title != "" {
		s.Title = c.Title
	}
	if c.Description != "" {
		s.Description = c.Description
	}
	if c.Format != "" {
		s.Format = c.Format
	}
	if c.Component == "" && c.DefaultSource == "" && c.Property == "" && !c.HasOrder {
		return
	}
	if s.Extensions == nil {
		s.Extensions = map[string]any{}
	}
	if c.Component != "" {
		s.Extensions["x-clicky-component"] = c.Component
	}
	if c.DefaultSource != "" {
		s.Extensions["x-clicky-default-source"] = c.DefaultSource
	}
	if c.Property != "" {
		s.Extensions["x-clicky-property"] = c.Property
	}
	if c.HasOrder {
		s.Extensions["x-clicky-order"] = c.Order
	}
}

// schemaFromMap builds an OpenAPISchema from a SchemaDescriber's map, routing
// the standard keywords to fields and the rest to Extensions.
func schemaFromMap(m map[string]any) *OpenAPISchema {
	s := &OpenAPISchema{}
	for k, v := range m {
		str, _ := v.(string)
		switch k {
		case "type":
			s.Type = str
		case "title":
			s.Title = str
		case "format":
			s.Format = str
		case "description":
			s.Description = str
		default:
			if s.Extensions == nil {
				s.Extensions = map[string]any{}
			}
			s.Extensions[k] = v
		}
	}
	return s
}
