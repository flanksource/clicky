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
		Name:  fieldName,
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
		} else if part == "tree" {
			field.Format = api.FormatTree
			// Initialize tree options if not already set
			if field.TreeOptions == nil {
				field.TreeOptions = api.DefaultTreeOptions()
			}
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

		// Get field name from json tag or use field name (same logic as StructToRow)
		fieldName := field.Name
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			if parts := strings.Split(jsonTag, ","); parts[0] != "" {
				fieldName = parts[0]
			}
		}

		prettyField := ParsePrettyTag(fieldName, prettyTag)
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

		// Handle Pretty interface and pointer dereferencing
		var value string
		if fieldVal.CanInterface() {
			if pretty, ok := fieldVal.Interface().(api.Pretty); ok {
				text := pretty.Pretty()
				value = text.String() // Use plain text for CSV
			} else {
				// Use processFieldValue to handle pointers properly
				actualValue := processFieldValue(fieldVal)
				value = fmt.Sprintf("%v", actualValue)
			}
		} else {
			value = fmt.Sprintf("%v", fieldVal.Interface())
		}
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

	// First try to split on underscores and dashes
	words := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-'
	})

	// If we only got one word (no underscores/dashes), try camelCase splitting
	if len(words) == 1 {
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
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Check if this rune starts a new word
		if i > 0 && r >= 'A' && r <= 'Z' {
			// Look back to see if previous character was lowercase
			prevIsLower := i > 0 && runes[i-1] >= 'a' && runes[i-1] <= 'z'

			// Only split on uppercase if previous was lowercase (simple camelCase like firstName, userID)
			// This keeps acronyms together (HTTPRequest stays as one word)
			if prevIsLower {
				if current.Len() > 0 {
					words = append(words, current.String())
					current.Reset()
				}
			}
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

// safeDerefPointer safely dereferences a pointer value, returning the dereferenced value and whether it was nil
func safeDerefPointer(val reflect.Value) (reflect.Value, bool) {
	if val.Kind() != reflect.Ptr {
		return val, false // Not a pointer, return as-is
	}

	if val.IsNil() {
		return reflect.Value{}, true // Nil pointer
	}

	return val.Elem(), false // Dereferenced value
}

// processSliceElement handles slice elements that might be nil pointers
func processSliceElement(elem reflect.Value) (reflect.Value, bool) {
	// If it's a pointer, dereference it safely
	if elem.Kind() == reflect.Ptr {
		if elem.IsNil() {
			return reflect.Value{}, true // Nil element
		}
		return elem.Elem(), false
	}

	return elem, false // Not a pointer
}

// processFieldValue processes a field value, handling pointers and returning the appropriate value for FieldValue
func processFieldValue(fieldVal reflect.Value) interface{} {
	// Handle nil pointers
	if fieldVal.Kind() == reflect.Ptr && fieldVal.IsNil() {
		return nil
	}

	// Dereference pointers
	if fieldVal.Kind() == reflect.Ptr {
		fieldVal = fieldVal.Elem()
	}

	// Handle slices - dereference pointer elements
	if fieldVal.Kind() == reflect.Slice {
		result := make([]interface{}, fieldVal.Len())
		for i := 0; i < fieldVal.Len(); i++ {
			elem := fieldVal.Index(i)
			if elem.Kind() == reflect.Ptr {
				if elem.IsNil() {
					result[i] = nil
				} else {
					result[i] = elem.Elem().Interface()
				}
			} else {
				result[i] = elem.Interface()
			}
		}
		return result
	}

	// Handle maps - dereference pointer values
	if fieldVal.Kind() == reflect.Map {
		result := make(map[string]interface{})
		iter := fieldVal.MapRange()
		for iter.Next() {
			k := iter.Key()
			v := iter.Value()

			keyStr := fmt.Sprintf("%v", k.Interface())

			if v.Kind() == reflect.Ptr {
				if v.IsNil() {
					result[keyStr] = nil
				} else {
					result[keyStr] = v.Elem().Interface()
				}
			} else {
				result[keyStr] = v.Interface()
			}
		}
		return result
	}

	// Return the interface value
	if fieldVal.IsValid() {
		return fieldVal.Interface()
	}

	return nil
}

// ToPrettyDataWithFormatHint converts various input types to PrettyData with a format hint for slices
func ToPrettyDataWithFormatHint(data interface{}, formatHint string) (*api.PrettyData, error) {
	// Handle nil data at root level
	if data == nil {
		return &api.PrettyData{
			Schema:   &api.PrettyObject{Fields: []api.PrettyField{}},
			Values:   make(map[string]api.FieldValue),
			Tables:   make(map[string][]api.PrettyDataRow),
			Original: data,
		}, nil
	}

	// Check if already PrettyData
	if pd, ok := data.(*api.PrettyData); ok {
		return pd, nil
	}

	val := reflect.ValueOf(data)

	// Handle nil pointer at root level
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return &api.PrettyData{
			Schema:   &api.PrettyObject{Fields: []api.PrettyField{}},
			Values:   make(map[string]api.FieldValue),
			Tables:   make(map[string][]api.PrettyDataRow),
			Original: data,
		}, nil
	}

	// Safely dereference root level pointer
	val, _ = safeDerefPointer(val)

	// Handle slices/arrays - force format hint
	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		if formatHint == "table" {
			result, err := convertSliceToPrettyData(val)
			return result, err
		} else if formatHint == "tree" {
			// Check if items have tree structure, otherwise convert to table
			if hasTreeStructure(val) {

				return convertSliceToTreeData(val)
			} else {

				return convertSliceToPrettyData(val)
			}
		}
		return convertSliceToPrettyData(val)
	}

	// For non-slices, delegate to the regular function
	return ToPrettyData(data)
}

// ToPrettyData converts various input types to PrettyData
func ToPrettyData(data interface{}) (*api.PrettyData, error) {
	// Handle nil data at root level
	if data == nil {
		return &api.PrettyData{
			Schema:   &api.PrettyObject{Fields: []api.PrettyField{}},
			Values:   make(map[string]api.FieldValue),
			Tables:   make(map[string][]api.PrettyDataRow),
			Original: data,
		}, nil
	}

	// Check if already PrettyData
	if pd, ok := data.(*api.PrettyData); ok {
		return pd, nil
	}

	// Check if data implements TreeNode interface first (more specific than Pretty)
	if treeNode, ok := data.(api.TreeNode); ok {
		// Create a PrettyData representation for TreeNode objects
		return &api.PrettyData{
			Schema: &api.PrettyObject{
				Fields: []api.PrettyField{
					{
						Name:   "tree",
						Format: api.FormatTree,
						Label:  "Tree",
					},
				},
			},
			Values: map[string]api.FieldValue{
				"tree": {
					Value: treeNode, // Store the TreeNode object
					Field: api.PrettyField{
						Name:   "tree",
						Format: api.FormatTree,
						Label:  "Tree",
					},
				},
			},
			Tables:   make(map[string][]api.PrettyDataRow),
			Original: data,
		}, nil
	}

	// Check if data implements Pretty interface
	if pretty, ok := data.(api.Pretty); ok {
		// Create a PrettyData representation for Pretty objects
		_ = pretty.Pretty() // We don't need the text here, just detect the interface
		return &api.PrettyData{
			Schema: &api.PrettyObject{
				Fields: []api.PrettyField{
					{
						Name:   "content",
						Format: "pretty", // Special format for Pretty objects
						Label:  "Content",
					},
				},
			},
			Values: map[string]api.FieldValue{
				"content": {
					Value: data, // Store the original Pretty object
					Field: api.PrettyField{
						Name:   "content",
						Format: "pretty",
						Label:  "Content",
					},
				},
			},
			Tables:   make(map[string][]api.PrettyDataRow),
			Original: data,
		}, nil
	}

	// Parse the input data
	val := reflect.ValueOf(data)

	// Handle nil pointer at root level
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return &api.PrettyData{
			Schema:   &api.PrettyObject{Fields: []api.PrettyField{}},
			Values:   make(map[string]api.FieldValue),
			Tables:   make(map[string][]api.PrettyDataRow),
			Original: data,
		}, nil
	}

	// Safely dereference root level pointer
	val, _ = safeDerefPointer(val)

	// Check dereferenced value for Pretty interface
	if val.CanInterface() {
		if pretty, ok := val.Interface().(api.Pretty); ok {
			// Create a PrettyData representation for Pretty objects
			_ = pretty.Pretty() // We don't need the text here, just detect the interface
			return &api.PrettyData{
				Schema: &api.PrettyObject{
					Fields: []api.PrettyField{
						{
							Name:   "content",
							Format: "pretty",
							Label:  "Content",
						},
					},
				},
				Values: map[string]api.FieldValue{
					"content": {
						Value: val.Interface(), // Store the dereferenced Pretty object
						Field: api.PrettyField{
							Name:   "content",
							Format: "pretty",
							Label:  "Content",
						},
					},
				},
				Tables:   make(map[string][]api.PrettyDataRow),
				Original: data,
			}, nil
		}
	}

	// Handle slices/arrays - default to table format unless items have tree structure
	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		if hasTreeStructure(val) {
			return convertSliceToTreeData(val)
		}
		return convertSliceToPrettyData(val)
	}

	// Create the schema from struct tags
	schema, err := ParseStructSchema(val)
	if err != nil {
		return nil, fmt.Errorf("failed to parse struct schema: %w", err)
	}

	// Create PrettyData from the schema and values
	prettyData := &api.PrettyData{
		Schema:   schema,
		Values:   make(map[string]api.FieldValue),
		Tables:   make(map[string][]api.PrettyDataRow),
		Original: data,
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
				elem := fieldVal.Index(i)

				// Handle nil elements in slice
				processedElem, isNil := processSliceElement(elem)
				if isNil {
					// Skip nil elements in table - they don't contribute to rows
					continue
				}

				row, err := StructToRow(processedElem)
				if err != nil {
					// Skip elements that can't be converted to rows
					continue
				}
				rows = append(rows, row)
			}
			prettyData.Tables[field.Name] = rows
		} else {
			// Regular field value - use processFieldValue to handle pointers
			prettyData.Values[field.Name] = api.FieldValue{
				Value: processFieldValue(fieldVal),
				Field: field,
			}
		}
	}

	return prettyData, nil
}

// hasTreeStructure checks if a slice contains items with tree-like fields
func hasTreeStructure(val reflect.Value) bool {
	if val.Len() == 0 {
		return false
	}

	// Get the first element to check the type
	firstElem := val.Index(0)
	firstElem, _ = safeDerefPointer(firstElem)

	if firstElem.Kind() != reflect.Struct {
		return false
	}

	// Check if any field has tree format or children-like field
	elemType := firstElem.Type()
	for i := 0; i < elemType.NumField(); i++ {
		field := elemType.Field(i)
		prettyTag := field.Tag.Get("pretty")
		if strings.Contains(prettyTag, "tree") || strings.Contains(prettyTag, "format=tree") {
			return true
		}
		// Check for common tree field names
		if strings.ToLower(field.Name) == "children" {
			return true
		}
	}

	return false
}

// convertSliceToTreeData converts a slice to tree-formatted PrettyData (placeholder)
func convertSliceToTreeData(val reflect.Value) (*api.PrettyData, error) {
	// For now, just delegate to table format
	// This can be expanded later to handle tree structures properly
	return convertSliceToPrettyData(val)
}

// convertSliceToPrettyData converts a slice/array to PrettyData with a table field
func convertSliceToPrettyData(val reflect.Value) (*api.PrettyData, error) {
	// Store the original interface value
	originalData := val.Interface()

	if val.Len() == 0 {
		// Empty slice - return empty PrettyData
		return &api.PrettyData{
			Schema:   &api.PrettyObject{Fields: []api.PrettyField{}},
			Values:   make(map[string]api.FieldValue),
			Tables:   make(map[string][]api.PrettyDataRow),
			Original: originalData,
		}, nil
	}

	// Get the first element to check the type
	firstElem := val.Index(0)
	firstElem, _ = safeDerefPointer(firstElem)

	// We only handle slices of structs
	if firstElem.Kind() != reflect.Struct {
		return nil, fmt.Errorf("can only convert slice of structs to PrettyData, got slice of %s", firstElem.Kind())
	}

	// Get the table schema from the first element
	tableFields, err := GetTableFields(firstElem)
	if err != nil {
		return nil, fmt.Errorf("failed to get table fields: %w", err)
	}

	// Convert all elements to rows
	var rows []api.PrettyDataRow
	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i)
		elem, isNil := safeDerefPointer(elem)
		if isNil {
			continue // Skip nil elements
		}

		row, err := StructToRow(elem)
		if err != nil {
			continue // Skip elements that can't be converted
		}
		rows = append(rows, row)
	}

	// Sort rows based on sort tags in the struct
	if len(rows) > 0 {
		sortFields := ExtractSortFields(firstElem.Type())
		if len(sortFields) > 0 {
			SortRows(rows, sortFields)
		}
	}

	// Create PrettyData with a single table field

	return &api.PrettyData{
		Schema: &api.PrettyObject{
			Fields: []api.PrettyField{
				{
					Name:   "data",
					Format: api.FormatTable,
					Label:  "Data",
					Fields: tableFields,
				},
			},
		},
		Values: make(map[string]api.FieldValue),
		Tables: map[string][]api.PrettyDataRow{
			"data": rows,
		},
		Original: originalData,
	}, nil
}

// StructToRow converts a struct to a PrettyDataRow
func StructToRow(val reflect.Value) (api.PrettyDataRow, error) {
	// Use our utility to safely dereference pointers
	val, isNil := safeDerefPointer(val)
	if isNil {
		return nil, fmt.Errorf("cannot convert nil pointer to row")
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

		// Get field name from json tag or use field name
		fieldName := field.Name
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			if parts := strings.Split(jsonTag, ","); parts[0] != "" {
				fieldName = parts[0]
			}
		}

		fieldVal := val.Field(i)
		prettyField := ParsePrettyTag(fieldName, prettyTag)

		// Use processFieldValue to handle pointer fields consistently
		row[fieldName] = api.FieldValue{
			Value: processFieldValue(fieldVal),
			Field: prettyField,
		}
	}

	return row, nil
}
