package formatters

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/flanksource/clicky/api"
)

// StructParser handles parsing of struct tags and schema extraction
type StructParser struct{}

// NewStructParser creates a new struct parser
func NewStructParser() *StructParser {
	return &StructParser{}
}

// ParsePrettyTag parses a pretty tag string into a PrettyField
func ParsePrettyTag(fieldName string, tag string) api.PrettyField {
	field := api.PrettyField{
		Name: fieldName,
		Label: fieldName, // Default label to field name
	}
	
	if tag == "" {
		return field
	}
	
	parts := strings.Split(tag, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		
		if strings.HasPrefix(part, "label=") {
			field.Label = strings.TrimPrefix(part, "label=")
		} else if strings.HasPrefix(part, "format=") {
			field.Format = strings.TrimPrefix(part, "format=")
		} else if strings.HasPrefix(part, "color=") || strings.Contains(part, "color") {
			field.Color = part
		} else if part == "table" {
			field.Format = api.FormatTable
		} else if part == "struct" {
			field.Format = "struct"
		} else if strings.HasPrefix(part, "title=") {
			field.TableOptions.Title = strings.TrimPrefix(part, "title=")
		} else if strings.HasPrefix(part, "sort=") {
			field.TableOptions.SortField = strings.TrimPrefix(part, "sort=")
		} else if strings.HasPrefix(part, "dir=") {
			field.TableOptions.SortDirection = strings.TrimPrefix(part, "dir=")
		} else if part == api.FormatHide || part == "hide" {
			field.Format = api.FormatHide
		}
	}
	
	// Default label to field name if not specified
	if field.Label == "" {
		field.Label = fieldName
	}
	
	return field
}

// ParseStructSchema creates a PrettyObject schema from struct tags
func ParseStructSchema(val reflect.Value) (*api.PrettyObject, error) {
	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %s", val.Kind())
	}
	
	typ := val.Type()
	obj := &api.PrettyObject{
		Fields: []api.PrettyField{},
	}
	
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		
		// Skip unexported fields
		if !field.IsExported() {
			continue
		}
		
		// Parse pretty tag
		prettyTag := field.Tag.Get("pretty")
		if prettyTag == "-" || prettyTag == api.FormatHide || prettyTag == "hide" {
			continue
		}
		
		prettyField := ParsePrettyTag(field.Name, prettyTag)
		
		// Check if it's a table field (slice/array of structs)
		fieldVal := val.Field(i)
		if strings.Contains(prettyTag, "table") && (fieldVal.Kind() == reflect.Slice || fieldVal.Kind() == reflect.Array) {
			prettyField.Format = api.FormatTable
			// Parse table schema from first element if available
			if fieldVal.Len() > 0 {
				firstElem := fieldVal.Index(0)
				if firstElem.Kind() == reflect.Ptr {
					firstElem = firstElem.Elem()
				}
				if firstElem.Kind() == reflect.Struct {
					tableFields, err := GetTableFields(firstElem)
					if err == nil {
						prettyField.Fields = tableFields
					}
				}
			}
		}
		
		obj.Fields = append(obj.Fields, prettyField)
	}
	
	return obj, nil
}

// GetTableFields extracts fields from a struct for table formatting
func GetTableFields(val reflect.Value) ([]api.PrettyField, error) {
	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct for table row, got %s", val.Kind())
	}
	
	typ := val.Type()
	var fields []api.PrettyField
	
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		
		// Skip unexported fields
		if !field.IsExported() {
			continue
		}
		
		// Parse pretty tag
		prettyTag := field.Tag.Get("pretty")
		if prettyTag == "-" || prettyTag == api.FormatHide || prettyTag == "hide" {
			continue
		}
		
		prettyField := ParsePrettyTag(field.Name, prettyTag)
		fields = append(fields, prettyField)
	}
	
	return fields, nil
}

// GetStructHeaders extracts field names as headers from structs, respecting pretty tags
func GetStructHeaders(val reflect.Value) []string {
	typ := val.Type()
	var headers []string
	
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)
		
		if !fieldVal.CanInterface() {
			continue
		}
		
		// Skip hidden fields
		prettyTag := field.Tag.Get("pretty")
		if prettyTag == api.FormatHide || prettyTag == "hide" || prettyTag == "-" {
			continue
		}
		
		// Get field name from json tag or use field name
		fieldName := field.Name
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			if parts := strings.Split(jsonTag, ","); parts[0] != "" {
				fieldName = parts[0]
			}
		}
		
		headers = append(headers, fieldName)
	}
	
	return headers
}

// GetStructRow extracts field values as a row from structs, respecting pretty tags
func GetStructRow(val reflect.Value) []string {
	typ := val.Type()
	var row []string
	
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)
		
		if !fieldVal.CanInterface() {
			continue
		}
		
		// Skip hidden fields
		prettyTag := field.Tag.Get("pretty")
		if prettyTag == api.FormatHide || prettyTag == "hide" || prettyTag == "-" {
			continue
		}
		
		// Convert value to string
		value := fmt.Sprintf("%v", fieldVal.Interface())
		row = append(row, value)
	}
	
	return row
}

// GetFieldValue gets a field value by name from a struct
func GetFieldValue(val reflect.Value, fieldName string) reflect.Value {
	typ := val.Type()
	
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		
		// Check field name
		if field.Name == fieldName {
			return val.Field(i)
		}
		
		// Check json tag
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			if parts := strings.Split(jsonTag, ","); parts[0] == fieldName {
				return val.Field(i)
			}
		}
	}
	
	// Return zero value if not found
	return reflect.Value{}
}

// GetFieldValueCaseInsensitive tries to find a field by name with different casing
func GetFieldValueCaseInsensitive(val reflect.Value, name string) reflect.Value {
	if val.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	
	typ := val.Type()
	// Try exact match first
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Name == name {
			return val.Field(i)
		}
	}
	
	// Try case-insensitive match
	lowerName := strings.ToLower(name)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if strings.ToLower(field.Name) == lowerName {
			return val.Field(i)
		}
	}
	
	return reflect.Value{}
}

// PrettifyFieldName converts field names to readable format
func PrettifyFieldName(name string) string {
	// Convert snake_case and camelCase to Title Case
	var result strings.Builder
	words := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-'
	})
	
	if len(words) == 0 {
		// Handle camelCase
		words = SplitCamelCase(name)
	}
	
	for i, word := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(strings.Title(strings.ToLower(word)))
	}
	
	return result.String()
}

// SplitCamelCase splits camelCase strings into words
func SplitCamelCase(s string) []string {
	var words []string
	var current strings.Builder
	
	for i, r := range s {
		if i > 0 && (r >= 'A' && r <= 'Z') {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		}
		current.WriteRune(r)
	}
	
	if current.Len() > 0 {
		words = append(words, current.String())
	}
	
	return words
}

// ToPrettyData converts various input types to PrettyData
func ToPrettyData(data interface{}) (*api.PrettyData, error) {
	// Check if already PrettyData
	if pd, ok := data.(*api.PrettyData); ok {
		return pd, nil
	}
	
	// Parse the struct and create PrettyData
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	
	// Create the schema from struct tags
	schema, err := ParseStructSchema(val)
	if err != nil {
		return nil, fmt.Errorf("failed to parse struct schema: %w", err)
	}
	
	// Create PrettyData from the schema and values
	prettyData := &api.PrettyData{
		Schema: schema,
		Values: make(map[string]api.FieldValue),
		Tables: make(map[string][]api.PrettyDataRow),
	}
	
	// Process each field
	for _, field := range schema.Fields {
		// Try both the field name as-is and with title case
		fieldVal := GetFieldValue(val, field.Name)
		if !fieldVal.IsValid() {
			// Try with different casing
			fieldVal = GetFieldValueCaseInsensitive(val, field.Name)
			if !fieldVal.IsValid() {
				continue
			}
		}
		
		// Handle table fields
		if field.Format == api.FormatTable && (fieldVal.Kind() == reflect.Slice || fieldVal.Kind() == reflect.Array) {
			// Convert slice to table rows
			var rows []api.PrettyDataRow
			for i := 0; i < fieldVal.Len(); i++ {
				row, err := StructToRow(fieldVal.Index(i))
				if err != nil {
					continue
				}
				rows = append(rows, row)
			}
			prettyData.Tables[field.Name] = rows
		} else {
			// Regular field value
			prettyData.Values[field.Name] = api.FieldValue{
				Value: fieldVal.Interface(),
				Field: field,
			}
		}
	}
	
	return prettyData, nil
}

// StructToRow converts a struct to a PrettyDataRow
func StructToRow(val reflect.Value) (api.PrettyDataRow, error) {
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	
	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %s", val.Kind())
	}
	
	row := make(api.PrettyDataRow)
	typ := val.Type()
	
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		
		// Skip unexported fields
		if !field.IsExported() {
			continue
		}
		
		// Skip fields with pretty:"-"
		prettyTag := field.Tag.Get("pretty")
		if prettyTag == "-" || prettyTag == api.FormatHide || prettyTag == "hide" {
			continue
		}
		
		fieldVal := val.Field(i)
		prettyField := ParsePrettyTag(field.Name, prettyTag)
		
		row[field.Name] = api.FieldValue{
			Value: fieldVal.Interface(),
			Field: prettyField,
		}
	}
	
	return row, nil
}