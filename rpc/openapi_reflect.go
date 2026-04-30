package rpc

import (
	"reflect"
	"strings"
	"time"
)

var timeType = reflect.TypeOf(time.Time{})

func (g *OpenAPIGenerator) convertGoTypeToOpenAPI(t reflect.Type) *OpenAPISchema {
	if t == nil {
		return &OpenAPISchema{Type: "object"}
	}

	for t.Kind() == reflect.Ptr {
		schema := g.convertGoTypeToOpenAPI(t.Elem())
		schema.Nullable = true
		return schema
	}

	if t == timeType {
		return &OpenAPISchema{Type: "string", Format: "date-time"}
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
			Items: g.convertGoTypeToOpenAPI(t.Elem()),
		}
	case reflect.Map:
		return &OpenAPISchema{
			Type:                 "object",
			AdditionalProperties: g.convertGoTypeToOpenAPI(t.Elem()),
		}
	case reflect.Struct:
		return g.convertStructTypeToOpenAPI(t)
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

func (g *OpenAPIGenerator) convertStructTypeToOpenAPI(t reflect.Type) *OpenAPISchema {
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
			embedded := g.convertStructTypeToOpenAPI(fieldType)
			for key, value := range embedded.Properties {
				schema.Properties[key] = value
			}
			schema.Required = append(schema.Required, embedded.Required...)
			continue
		}
		if name == "" {
			name = field.Name
		}

		schema.Properties[name] = g.convertGoTypeToOpenAPI(field.Type)
		if !omitempty && !isOptionalResponseField(field.Type) {
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
