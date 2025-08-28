package formatters

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/flanksource/clicky/api"
)

// NewStructParser creates a new struct parser
func NewStructParser() *api.StructParser {
	return api.NewStructParser()
}

// ParsePrettyTag parses a pretty tag string into a PrettyField
// Deprecated: Use api.ParsePrettyTagWithName instead
func ParsePrettyTag(fieldName, tag string) api.PrettyField {
	return api.ParsePrettyTagWithName(fieldName, tag)
}

// ParseStructSchema creates a PrettyObject schema from struct tags
// Deprecated: Use api.StructParser.ParseStructSchema instead
func ParseStructSchema(val reflect.Value) (*api.PrettyObject, error) {
	parser := api.NewStructParser()
	return parser.ParseStructSchema(val)
}

// GetTableFields extracts fields from a struct for table formatting
// Deprecated: Use api.StructParser.GetTableFields instead
func GetTableFields(val reflect.Value) ([]api.PrettyField, error) {
	parser := api.NewStructParser()
	return parser.GetTableFields(val)
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
// Deprecated: Use api.StructParser.GetFieldValue instead
func GetFieldValue(val reflect.Value, fieldName string) reflect.Value {
	parser := api.NewStructParser()
	return parser.GetFieldValue(val, fieldName)
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
		if strings.EqualFold(field.Name, lowerName) {
			return val.Field(i)
		}
	}

	return reflect.Value{}
}

// PrettifyFieldName converts field names to readable format
// Deprecated: Use api.PrettifyFieldName instead
func PrettifyFieldName(name string) string {
	return api.PrettifyFieldName(name)
}

// SplitCamelCase splits camelCase strings into words
// Deprecated: Use api.SplitCamelCase instead
func SplitCamelCase(s string) []string {
	return api.SplitCamelCase(s)
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
	parser := api.NewStructParser()
	return parser.ProcessFieldValue(fieldVal)
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
		if strings.EqualFold(field.Name, "children") {
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
// Deprecated: Use api.StructParser.StructToRow instead
func StructToRow(val reflect.Value) (api.PrettyDataRow, error) {
	parser := api.NewStructParser()
	return parser.StructToRow(val)
}
