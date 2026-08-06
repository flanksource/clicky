package entity

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"unicode"
)

// jsonSchema is the subset of JSON Schema clicky reads for a dynamic entity,
// plus the x-clicky-* extension keywords that drive entity semantics.
type jsonSchema struct {
	Type       string                     `json:"type"`
	Properties map[string]*schemaProperty `json:"properties"`
	Required   []string                   `json:"required"`
	Aliases    []string                   `json:"x-clicky-aliases"`
	Parent     string                     `json:"x-clicky-parent"`
	Icon       string                     `json:"x-clicky-icon"`
	Path       string                     `json:"x-clicky-path"`
	Title      string                     `json:"x-clicky-title"`
}

// schemaProperty is one property's schema plus its x-clicky-* annotations.
type schemaProperty struct {
	Type        string          `json:"type"`
	Format      string          `json:"format"`
	Description string          `json:"description"`
	Items       *schemaProperty `json:"items"`
	IsID        bool            `json:"x-clicky-id"`
	IsName      bool            `json:"x-clicky-name"`
	Filter      string          `json:"x-clicky-filter"`
	FilterKey   string          `json:"x-clicky-filter-key"`
	Label       string          `json:"x-clicky-label"`
	PrettyFmt   string          `json:"x-clicky-format"`
	Short       bool            `json:"x-clicky-short"`
}

// schemaField is a flattened, ordered property descriptor.
type schemaField struct {
	Name      string // JSON property name / flag key
	GoName    string // exported Go field name for reflect.StructOf
	JSONType  string
	ItemType  string // element type when JSONType == "array"
	Format    string
	Label     string
	PrettyFmt string
	Short     bool
	Filter    string // referenced named filter; "" => not filterable
	FilterKey string // bound CLI/query parameter; defaults to Name
}

func (f schemaField) isArray() bool { return f.JSONType == "array" }

// parsedSchema is the validated, ordered result of parsing a dynamic entity schema.
type parsedSchema struct {
	Aliases []string
	Parent  string
	Icon    string
	Path    string
	Title   string
	IDKey   string
	NameKey string
	Fields  []schemaField
}

// parseSchema parses and validates a dynamic entity JSON schema. Fields are
// ordered deterministically (by name) so generated structs and CLI flags are
// stable across runs.
func parseSchema(raw []byte) (*parsedSchema, error) {
	var doc jsonSchema
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse dynamic entity schema: %w", err)
	}
	if len(doc.Properties) == 0 {
		return nil, fmt.Errorf("dynamic entity schema has no properties")
	}

	names := make([]string, 0, len(doc.Properties))
	for name := range doc.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	ps := &parsedSchema{Aliases: doc.Aliases, Parent: doc.Parent, Icon: doc.Icon, Path: doc.Path, Title: doc.Title}
	seen := map[string]int{}
	for _, name := range names {
		p := doc.Properties[name]
		if p == nil {
			return nil, fmt.Errorf("dynamic entity schema property %q must be an object, got null", name)
		}
		goName, err := uniqueGoFieldName(name, seen)
		if err != nil {
			return nil, err
		}
		field := schemaField{
			Name:      name,
			GoName:    goName,
			JSONType:  p.Type,
			Format:    p.Format,
			Label:     p.Label,
			PrettyFmt: p.PrettyFmt,
			Short:     p.Short,
			Filter:    p.Filter,
			FilterKey: p.FilterKey,
		}
		if p.Items != nil {
			field.ItemType = p.Items.Type
		}
		if field.Filter != "" && field.FilterKey == "" {
			field.FilterKey = name
		}
		if p.IsID {
			if ps.IDKey != "" {
				return nil, fmt.Errorf("dynamic entity schema has multiple x-clicky-id properties: %q and %q", ps.IDKey, name)
			}
			ps.IDKey = name
		}
		if p.IsName {
			if ps.NameKey != "" {
				return nil, fmt.Errorf("dynamic entity schema has multiple x-clicky-name properties: %q and %q", ps.NameKey, name)
			}
			ps.NameKey = name
		}
		ps.Fields = append(ps.Fields, field)
	}

	if ps.IDKey == "" {
		return nil, fmt.Errorf("dynamic entity schema must mark one property with x-clicky-id")
	}
	if ps.NameKey == "" {
		ps.NameKey = ps.IDKey
	}
	return ps, nil
}

// listType builds the runtime ListOpts struct (filterable fields only) so the
// existing CLI flag binding and OpenAPI parameter generation work unchanged.
func (ps *parsedSchema) listType() reflect.Type {
	var fields []reflect.StructField
	for _, f := range ps.Fields {
		if f.Filter == "" {
			continue
		}
		fieldType := reflect.TypeOf("")
		if f.isArray() {
			fieldType = reflect.TypeOf([]string(nil))
		}
		fields = append(fields, reflect.StructField{
			Name: f.GoName,
			Type: fieldType,
			Tag:  reflect.StructTag(fmt.Sprintf("flag:%q json:%q", f.FilterKey, f.FilterKey)),
		})
	}
	return reflect.StructOf(fields)
}

// itemType builds the runtime item struct (all fields) used only for OpenAPI
// response-schema generation; actual rows are map-backed.
func (ps *parsedSchema) itemType() reflect.Type {
	fields := make([]reflect.StructField, 0, len(ps.Fields))
	for _, f := range ps.Fields {
		fields = append(fields, reflect.StructField{
			Name: f.GoName,
			Type: goType(f.JSONType, f.ItemType),
			Tag:  reflect.StructTag(itemFieldTag(f)),
		})
	}
	return reflect.StructOf(fields)
}

func itemFieldTag(f schemaField) string {
	tag := fmt.Sprintf("json:%q", f.Name)
	if f.PrettyFmt != "" || f.Label != "" || f.Short {
		pretty := f.Name
		if f.Label != "" {
			pretty = "label=" + f.Label
		}
		if f.PrettyFmt != "" {
			pretty += ",format=" + f.PrettyFmt
		}
		if f.Short {
			pretty += ",short"
		}
		tag += fmt.Sprintf(" pretty:%q", pretty)
	}
	return tag
}

func goType(jsonType, itemType string) reflect.Type {
	switch jsonType {
	case "integer":
		return reflect.TypeOf(int64(0))
	case "number":
		return reflect.TypeOf(float64(0))
	case "boolean":
		return reflect.TypeOf(false)
	case "array":
		return reflect.SliceOf(goType(itemType, ""))
	case "object":
		return reflect.TypeOf(map[string]any{})
	default:
		return reflect.TypeOf("")
	}
}

func uniqueGoFieldName(name string, seen map[string]int) (string, error) {
	base := goFieldName(name)
	if base == "" {
		return "", fmt.Errorf("property %q does not yield a valid Go field name", name)
	}
	seen[base]++
	if n := seen[base]; n > 1 {
		base += strconv.Itoa(n)
	}
	return base, nil
}

// goFieldName converts a JSON property name to an exported Go identifier,
// upper-casing the first letter and any letter following a separator.
func goFieldName(name string) string {
	var out []rune
	upNext := true
	for _, r := range name {
		switch {
		case unicode.IsLetter(r):
			if upNext {
				r = unicode.ToUpper(r)
			}
			out = append(out, r)
			upNext = false
		case unicode.IsDigit(r):
			if len(out) == 0 {
				out = append(out, 'F') // identifiers can't start with a digit
			}
			out = append(out, r)
			upNext = false
		default:
			upNext = true
		}
	}
	return string(out)
}
