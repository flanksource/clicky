package formatters

import (
	"reflect"
	"strings"

	"github.com/flanksource/clicky/api"
)

// PrettyFormatter handles formatting of structs with pretty tags
type PrettyFormatter struct {
	Theme   api.Theme
	NoColor bool
	parser  *api.StructParser
}

// NewPrettyFormatter creates a new formatter with adaptive theme
func NewPrettyFormatter() *PrettyFormatter {
	return &PrettyFormatter{
		Theme:  api.AutoTheme(),
		parser: api.NewStructParser(),
	}
}

// NewPrettyFormatterWithTheme creates a new formatter with a specific theme
func NewPrettyFormatterWithTheme(theme api.Theme) *PrettyFormatter {
	return &PrettyFormatter{
		Theme:  theme,
		parser: api.NewStructParser(),
	}
}

func (p *PrettyFormatter) Parse(data interface{}) (*api.PrettyData, error) {
	schema, err := p.parser.Parse(data)
	if err != nil {
		return nil, err
	}

	prettyData, err := p.parser.ParseDataWithSchema(data, schema)
	if err != nil {
		return nil, err
	}
	prettyData.Schema = schema
	prettyData.Original = data

	theme := p.Theme
	if p.NoColor {
		theme = api.NoTTYTheme()
	}
	applyRenderFuncs(prettyData, data, schema, theme)

	return prettyData, nil
}

func applyRenderFuncs(prettyData *api.PrettyData, raw interface{}, schema *api.PrettyObject, theme api.Theme) {
	if prettyData == nil || schema == nil || prettyData.TypedMap == nil {
		return
	}

	applyRenderFuncsToTypedMap(prettyData.TypedMap, raw, schema.Fields, theme)
}

func applyRenderFuncsToTypedMap(typedMap *api.TypedMap, raw interface{}, fields []api.PrettyField, theme api.Theme) {
	if typedMap == nil {
		return
	}

	rawVal := reflect.ValueOf(raw)
	for rawVal.IsValid() && (rawVal.Kind() == reflect.Ptr || rawVal.Kind() == reflect.Interface) {
		if rawVal.IsNil() {
			return
		}
		rawVal = rawVal.Elem()
	}

	if !rawVal.IsValid() || (rawVal.Kind() != reflect.Struct && rawVal.Kind() != reflect.Map) {
		return
	}

	for _, field := range fields {
		fieldVal, ok := getFieldValueWithAliases(rawVal, field)
		if !ok {
			continue
		}

		if field.RenderFunc != nil {
			renderVal := fieldVal
			for renderVal.IsValid() && (renderVal.Kind() == reflect.Ptr || renderVal.Kind() == reflect.Interface) {
				if renderVal.IsNil() {
					renderVal = reflect.Value{}
					break
				}
				renderVal = renderVal.Elem()
			}
			if renderVal.IsValid() {
				rendered := field.RenderFunc(renderVal.Interface(), field, theme)
				(*typedMap)[field.Name] = api.TypedValue{Textable: api.Text{Content: rendered}}
			}
		}

		if len(field.Fields) > 0 {
			if nested, exists := (*typedMap)[field.Name]; exists && nested.TypedMap != nil && fieldVal.IsValid() {
				applyRenderFuncsToTypedMap(nested.TypedMap, fieldVal.Interface(), field.Fields, theme)
			}
		}
	}
}

func getFieldValueWithAliases(val reflect.Value, field api.PrettyField) (reflect.Value, bool) {
	names := make([]string, 0, len(field.Aliases)+1)
	names = append(names, field.Aliases...)
	if field.Name != "" {
		names = append(names, field.Name)
	}

	switch val.Kind() {
	case reflect.Map:
		return getMapValueWithAliases(val, names)
	case reflect.Struct:
		return getStructFieldValueWithAliases(val, names)
	default:
		return reflect.Value{}, false
	}
}

func getMapValueWithAliases(val reflect.Value, names []string) (reflect.Value, bool) {
	if val.Kind() != reflect.Map {
		return reflect.Value{}, false
	}

	keyType := val.Type().Key()
	for _, name := range names {
		if name == "" {
			continue
		}

		if keyType.Kind() == reflect.String {
			mapVal := val.MapIndex(reflect.ValueOf(name))
			if mapVal.IsValid() {
				return mapVal, true
			}
		}

		iter := val.MapRange()
		for iter.Next() {
			key := iter.Key()
			if key.Kind() == reflect.String && strings.EqualFold(key.String(), name) {
				return iter.Value(), true
			}
		}
	}

	return reflect.Value{}, false
}

func getStructFieldValueWithAliases(val reflect.Value, names []string) (reflect.Value, bool) {
	if val.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		jsonName := ""
		if jsonTag != "" && jsonTag != "-" {
			if parts := strings.Split(jsonTag, ","); parts[0] != "" {
				jsonName = parts[0]
			}
		}

		for _, name := range names {
			if name == "" {
				continue
			}
			if field.Name == name || jsonName == name {
				return val.Field(i), true
			}
		}
	}

	return reflect.Value{}, false
}

// Format formats data and returns formatted output
func (p *PrettyFormatter) Format(data interface{}) (string, error) {
	// Check if this is already parsed PrettyData
	if prettyData, ok := data.(*api.PrettyData); ok {
		return p.FormatPrettyData(prettyData)
	}
	prettyData, err := ToPrettyData(data)
	if err != nil {
		return "", err
	}
	return p.FormatPrettyData(prettyData)
}

// FormatPrettyData formats PrettyData structure
func (p *PrettyFormatter) FormatPrettyData(data *api.PrettyData) (string, error) {
	if data == nil {
		return "", nil
	}

	if p.NoColor {
		return data.String(), nil
	}
	return data.ANSI(), nil
}
