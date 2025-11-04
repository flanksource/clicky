package formatters

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/commons/logger"
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
func GetStructRow(val reflect.Value) api.TextList {
	typ := val.Type()
	var row api.TextList

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
		if fieldVal.CanInterface() {
			if pretty, ok := fieldVal.Interface().(api.Pretty); ok {
				row = append(row, pretty.Pretty())
			} else {
				// Use processFieldValue to handle pointers properly
				actualValue := processFieldValue(fieldVal)
				value := fmt.Sprintf("%v", actualValue)
				row = append(row, api.Text{Content: value})
			}
		} else {
			value := fmt.Sprintf("%v", fieldVal.Interface())
			row = append(row, api.Text{Content: value})
		}
	}

	return row
}

// GetFieldValue gets a field value by name from a struct
// Deprecated: Use api.StructParser.GetFieldValue instead
func GetFieldValue(val reflect.Value, fieldName string) reflect.Value {
	parser := api.NewStructParser()
	return parser.GetFieldValue(val, fieldName)
}

// GetFieldValueWithAliases tries to get a field value using aliases first, then the field name
func GetFieldValueWithAliases(val reflect.Value, field api.PrettyField) reflect.Value {
	// Try aliases first
	if len(field.Aliases) > 0 {
		for _, alias := range field.Aliases {
			fieldVal := GetFieldValue(val, alias)
			if fieldVal.IsValid() && !isEmptyValue(fieldVal) {
				return fieldVal
			}
		}
	}

	// Fall back to the field name
	fieldVal := GetFieldValue(val, field.Name)
	if !fieldVal.IsValid() {
		// Try with different casing
		fieldVal = GetFieldValueCaseInsensitive(val, field.Name)
	}

	return fieldVal
}

// isEmptyValue checks if a reflect.Value is considered empty
func isEmptyValue(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Chan:
		return v.Len() == 0
	case reflect.Interface:
		if v.IsNil() {
			return true
		}
		// For interface{}, check the underlying value
		return isEmptyValue(v.Elem())
	case reflect.Ptr:
		return v.IsNil()
	default:
		return false
	}
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

// FlattenSlice flattens a slice of slices into a single-level slice.
// If the input is not a slice of slices, it returns the input unchanged.
// This allows safe use on any slice without pre-checking.
func FlattenSlice(val reflect.Value) reflect.Value {
	// Check if input is a slice or array
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return val
	}

	// Empty slice - return as-is
	if val.Len() == 0 {
		return val
	}

	// Get the first element to check if this is a slice of slices
	firstElem := val.Index(0)
	firstElem, _ = safeDerefPointer(firstElem)

	// Dereference interface to get underlying concrete type
	if firstElem.Kind() == reflect.Interface && !firstElem.IsNil() {
		firstElem = firstElem.Elem()
	}

	// Not a slice of slices - return input unchanged
	if firstElem.Kind() != reflect.Slice && firstElem.Kind() != reflect.Array {
		return val
	}

	// It's a slice of slices - flatten it
	var flattened []reflect.Value
	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i)
		elem, isNil := safeDerefPointer(elem)
		if isNil {
			continue // Skip nil outer elements
		}

		// Dereference interface
		if elem.Kind() == reflect.Interface && !elem.IsNil() {
			elem = elem.Elem()
		}

		// Iterate inner slice and collect all elements
		if elem.Kind() == reflect.Slice || elem.Kind() == reflect.Array {
			for j := 0; j < elem.Len(); j++ {
				innerElem := elem.Index(j)
				flattened = append(flattened, innerElem)
			}
		}
	}

	// If no elements were collected, return empty slice of same type as input
	if len(flattened) == 0 {
		return reflect.MakeSlice(val.Type(), 0, 0)
	}

	// Create a new slice with the flattened elements
	// Determine the element type from the first flattened element
	elemType := flattened[0].Type()
	newSlice := reflect.MakeSlice(reflect.SliceOf(elemType), len(flattened), len(flattened))
	for i, elem := range flattened {
		newSlice.Index(i).Set(elem)
	}

	return newSlice
}

// ToPrettyDataWithOptions converts various input types to PrettyData using format options
func ToPrettyDataWithOptions(data interface{}, opts FormatOptions) (*api.PrettyData, error) {
	// Handle nil data at root level
	if data == nil {
		return &api.PrettyData{
			Schema:   &api.PrettyObject{Fields: []api.PrettyField{}},
			Values:   make(map[string]api.FieldValue),
			Tables:   make(map[string][]api.PrettyDataRow),
			Original: data,
		}, nil
	}

	// Check if data implements Pretty interface first
	if pretty, ok := data.(api.Pretty); ok {
		// For Pretty objects, create a simple field value
		text := pretty.Pretty()
		return &api.PrettyData{
			Schema: &api.PrettyObject{Fields: []api.PrettyField{{Name: "content", Type: "string"}}},
			Values: map[string]api.FieldValue{
				"content": {
					Value: text.Content,
					Text:  &text,
					Field: api.PrettyField{Name: "content", Type: "string"},
				},
			},
			Tables:   make(map[string][]api.PrettyDataRow),
			Original: data,
		}, nil
	}

	// Get reflect value
	val := reflect.ValueOf(data)

	// Check if it's a slice/array
	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		return parseSliceDataWithOptions(val, opts)
	}

	// For single objects (struct or map)
	return parseStructDataWithOptions(val, opts)
}

// hasPrettyImplementers checks if slice elements implement api.Pretty interface
func hasPrettyImplementers(val reflect.Value) bool {
	if val.Len() == 0 {
		return false
	}

	// Check first few elements to see if they implement Pretty
	checkCount := val.Len()
	if checkCount > 3 {
		checkCount = 3
	}
	prettyCount := 0

	for i := 0; i < checkCount; i++ {
		elem := val.Index(i)
		elem, isNil := safeDerefPointer(elem)
		if isNil {
			continue
		}

		if elem.CanInterface() {
			if _, ok := elem.Interface().(api.Pretty); ok {
				prettyCount++
			}
		}
	}

	// If all checked elements implement Pretty, treat as Pretty slice
	return prettyCount == checkCount
}

// convertSliceToPrettyList converts a slice of Pretty implementers to PrettyData as a list
func convertSliceToPrettyList(val reflect.Value) (*api.PrettyData, error) {
	prettyData := &api.PrettyData{
		Schema:   &api.PrettyObject{Fields: []api.PrettyField{}},
		Values:   make(map[string]api.FieldValue),
		Tables:   make(map[string][]api.PrettyDataRow),
		Original: val.Interface(),
	}

	// Create a field that holds all the pretty items
	items := make([]api.Text, 0, val.Len())

	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i)
		elem, isNil := safeDerefPointer(elem)
		if isNil {
			continue
		}

		if elem.CanInterface() {
			if pretty, ok := elem.Interface().(api.Pretty); ok {
				items = append(items, pretty.Pretty())
			}
		}
	}

	// Store as a list field value
	prettyData.Schema.Fields = append(prettyData.Schema.Fields, api.PrettyField{
		Name:   "items",
		Format: "list",
		Label:  "Items",
	})

	prettyData.Values["items"] = api.FieldValue{
		Value: items,
		Field: api.PrettyField{
			Name:   "items",
			Format: "list",
		},
	}

	return prettyData, nil
}

// parseSliceDataWithOptions handles slice/array data with format options
func parseSliceDataWithOptions(val reflect.Value, opts FormatOptions) (*api.PrettyData, error) {
	// Safely dereference root level pointer
	val, _ = safeDerefPointer(val)
	val = FlattenSlice(val)

	// Handle slices/arrays - default to table format unless items have tree structure
	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		// If --table is explicitly set, force table format even for TreeNodes
		if opts.Table {
			return convertSliceToPrettyDataWithOptions(val, opts)
		}
		// Otherwise, detect tree structure and use tree format if applicable
		if hasTreeStructure(val) {
			return convertSliceToTreeData(val)
		}
		return convertSliceToPrettyDataWithOptions(val, opts)
	}

	// Not a slice, delegate to struct parser
	return parseStructDataWithOptions(val, opts)
}

// parseStructDataWithOptions handles struct/map data with format options
func parseStructDataWithOptions(val reflect.Value, opts FormatOptions) (*api.PrettyData, error) {
	// Safely dereference root level pointer
	val, _ = safeDerefPointer(val)

	// Check dereferenced value for Pretty interface
	if val.CanInterface() {
		if pretty, ok := val.Interface().(api.Pretty); ok {
			// For Pretty objects, create a simple field value
			text := pretty.Pretty()
			return &api.PrettyData{
				Schema: &api.PrettyObject{
					Fields: []api.PrettyField{{
						Name:   "content",
						Format: "pretty",
						Label:  "Content",
					}},
				},
				Values: map[string]api.FieldValue{
					"content": {
						Value: val.Interface(), // Store the dereferenced Pretty object
						Text:  &text,           // Store the pretty text
						Field: api.PrettyField{
							Name:   "content",
							Format: "pretty",
							Label:  "Content",
						},
					},
				},
				Tables:   make(map[string][]api.PrettyDataRow),
				Original: val.Interface(),
			}, nil
		}
	}

	// For maps, we need to manually create a schema and parse
	if val.Kind() == reflect.Map {
		// Create a schema from the map structure
		schema := &api.PrettyObject{Fields: []api.PrettyField{}}

		// Iterate over map keys to detect schema
		for _, key := range val.MapKeys() {
			if key.Kind() != reflect.String {
				continue
			}
			fieldName := key.String()
			fieldVal := val.MapIndex(key)

			// Handle interface{} wrapping
			if fieldVal.Kind() == reflect.Interface && !fieldVal.IsNil() {
				fieldVal = fieldVal.Elem()
			}

			// Detect if this field should be a table (slice of structs/maps)
			field := api.PrettyField{
				Name:  fieldName,
				Label: fieldName,
			}

			if fieldVal.Kind() == reflect.Slice || fieldVal.Kind() == reflect.Array {
				if fieldVal.Len() > 0 {
					firstElem := fieldVal.Index(0)
					if firstElem.Kind() == reflect.Interface && !firstElem.IsNil() {
						firstElem = firstElem.Elem()
					}
					if firstElem.Kind() == reflect.Map || firstElem.Kind() == reflect.Struct {
						field.Format = api.FormatTable

						// Extract table fields from the first element
						parser := api.NewStructParser()
						var tableFields []api.PrettyField
						if firstElem.Kind() == reflect.Map {
							// Get fields from map with proper type inference
							keys := firstElem.MapKeys()
							// Sort keys for consistent ordering
							sort.Slice(keys, func(i, j int) bool {
								return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
							})
							for _, key := range keys {
								if key.Kind() == reflect.String {
									keyStr := key.String()
									mapVal := firstElem.MapIndex(key)
									// Handle interface wrapping
									if mapVal.Kind() == reflect.Interface && !mapVal.IsNil() {
										mapVal = mapVal.Elem()
									}
									// Infer type from the value
									inferredType := "string" // default
									switch mapVal.Kind() {
									case reflect.Bool:
										inferredType = "bool"
									case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
										inferredType = "int"
									case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
										inferredType = "int"
									case reflect.Float32, reflect.Float64:
										inferredType = "float"
									}
									tableFields = append(tableFields, api.PrettyField{
										Name:  keyStr,
										Label: keyStr,
										Type:  inferredType,
									})
								}
							}
						} else {
							var err error
							tableFields, err = parser.GetTableFields(firstElem)
							if err != nil {
								logger.V(4).Infof("Failed to get table fields: %v", err)
							}
						}
						field.TableOptions = api.TableOptions{Columns: tableFields}
					}
				}
			}

			schema.Fields = append(schema.Fields, field)
		}

		// Parse the map data with the schema
		return parseStructDataWithOptionsAndSchema(val, schema, opts)
	}

	// Create the schema from struct tags
	schema, err := ParseStructSchema(val)
	if err != nil {
		return nil, fmt.Errorf("failed to parse struct schema: %w", err)
	}

	// Parse the struct data with options
	return parseStructDataWithOptionsAndSchema(val, schema, opts)
}

// convertSliceToPrettyDataWithOptions converts a slice to PrettyData using format options
func convertSliceToPrettyDataWithOptions(val reflect.Value, opts FormatOptions) (*api.PrettyData, error) {
	// Store the original interface value
	originalData := val.Interface()

	// Flatten slice of slices before processing
	val = FlattenSlice(val)

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

	// Dereference interface to get underlying concrete type
	if firstElem.Kind() == reflect.Interface && !firstElem.IsNil() {
		firstElem = firstElem.Elem()
	}

	// Recursively dereference all pointer layers until we reach a non-pointer type
	for firstElem.Kind() == reflect.Ptr {
		if firstElem.IsNil() {
			return nil, fmt.Errorf("cannot convert slice with nil pointer element")
		}
		firstElem = firstElem.Elem()
	}

	// Handle slices of structs or maps
	if firstElem.Kind() != reflect.Struct && firstElem.Kind() != reflect.Map {
		return nil, fmt.Errorf("can only convert slice of structs or maps to PrettyData, got slice of %s", firstElem.Kind())
	}

	// Convert all elements to rows using options-aware method
	var rows []api.PrettyDataRow
	var tableFields []api.PrettyField
	parser := api.NewStructParser()

	// For slices of maps, extract all distinct keys from all maps
	if firstElem.Kind() == reflect.Map {
		keysSet := make(map[string]bool)
		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i)
			elem, isNil := safeDerefPointer(elem)
			if isNil {
				continue
			}
			if elem.Kind() == reflect.Map {
				for _, key := range elem.MapKeys() {
					if key.Kind() == reflect.String {
						keysSet[key.String()] = true
					}
				}
			}
		}
		// Convert to sorted slice of fields
		keys := make([]string, 0, len(keysSet))
		for k := range keysSet {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			tableFields = append(tableFields, api.PrettyField{
				Name:  k,
				Label: k,
				Type:  "string",
			})
		}
	}

	// Statistics for PrettyRow usage
	prettyRowCount := 0
	reflectionCount := 0

	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i)
		elem, isNil := safeDerefPointer(elem)
		if isNil {
			continue // Skip nil elements
		}

		// Check if this element implements PrettyRow for statistics
		if elem.CanInterface() {
			if _, ok := elem.Interface().(api.PrettyRow); ok {
				prettyRowCount++
			} else {
				reflectionCount++
			}
		} else {
			reflectionCount++
		}

		// Use StructToRowWithOptions to check for PrettyRow interface
		row, err := parser.StructToRowWithOptions(elem, opts)
		if err != nil {
			logger.V(4).Infof("Failed to convert element of %T: %v: %v", elem, elem, err)
			continue // Skip elements that can't be converted
		}

		// Extract table schema from the first successful row (for structs)
		// This ensures schema matches PrettyRow columns
		if i == 0 && len(tableFields) == 0 {
			for columnName := range row {
				tableFields = append(tableFields, api.PrettyField{
					Name:  columnName,
					Label: columnName,
					Type:  "string", // Default type, could be enhanced
				})
			}
		}

		rows = append(rows, row)
	}

	// Fallback to struct reflection if no rows were generated (structs only)
	if len(rows) == 0 || (len(tableFields) == 0 && firstElem.Kind() == reflect.Struct) {
		var err error
		tableFields, err = GetTableFields(firstElem)
		if err != nil {
			return nil, fmt.Errorf("failed to get table fields: %w", err)
		}
	}

	// Sort rows based on sort tags in the struct
	if len(rows) > 0 {
		sortFields := ExtractSortFields(firstElem.Type())
		if len(sortFields) > 0 {
			SortRows(rows, sortFields)
		}
	}

	// Apply filter if provided in options
	if opts.Filter != "" && len(rows) > 0 {
		filteredRows, err := api.FilterTableRows(rows, opts.Filter)
		if err != nil {
			return nil, fmt.Errorf("failed to apply filter: %w", err)
		}
		rows = filteredRows
	}

	return &api.PrettyData{
		Schema: &api.PrettyObject{
			Fields: []api.PrettyField{
				{
					Name:         "table",
					Format:       api.FormatTable,
					TableOptions: api.TableOptions{Columns: tableFields},
				},
			},
		},
		Values: make(map[string]api.FieldValue),
		Tables: map[string][]api.PrettyDataRow{
			"table": rows,
		},
		Original: originalData,
	}, nil
}

// parseStructDataWithOptionsAndSchema parses struct data with schema and options
func parseStructDataWithOptionsAndSchema(val reflect.Value, schema *api.PrettyObject, opts FormatOptions) (*api.PrettyData, error) {
	// This would be similar to existing struct parsing but with options
	// For now, delegate to existing parsing since most struct parsing doesn't need special option handling
	parser := api.NewStructParser()
	prettyData, err := parser.ParseDataWithSchema(val.Interface(), schema)
	if err != nil {
		return nil, err
	}

	// Apply filter to all table fields if filter is provided
	if opts.Filter != "" && prettyData != nil && prettyData.Tables != nil {
		for tableName, rows := range prettyData.Tables {
			if len(rows) > 0 {
				filteredRows, err := api.FilterTableRows(rows, opts.Filter)
				if err != nil {
					// Skip tables where filter references non-existent fields
					logger.V(4).Infof("Skipping filter for table %s: %v", tableName, err)
					continue
				}
				prettyData.Tables[tableName] = filteredRows
			}
		}
	}

	return prettyData, nil
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

	// Check if data implements TreeMixin interface first (most specific)
	if treeMixin, ok := data.(api.TreeMixin); ok {
		treeNode := treeMixin.Tree()
		// Create a PrettyData representation for TreeMixin objects
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

	// Check if data implements TreeNode interface (direct tree node)
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

	val = FlattenSlice(val)
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
		// Try aliases first, then the field name
		fieldVal := GetFieldValueWithAliases(val, field)
		if !fieldVal.IsValid() {
			continue
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
		} else if field.Format == api.FormatTree {
			// Handle tree fields - convert to SimpleTreeNode for consistent formatting
			var treeNode api.TreeNode
			if tn, ok := fieldVal.Interface().(api.TreeNode); ok {
				treeNode = api.TreeNodeToSimple(tn)
			}

			if treeNode != nil {
				prettyData.Values[field.Name] = api.FieldValue{
					Value: treeNode,
					Field: field,
				}
			}
		} else if (field.Type == "map" || field.Type == "struct") && (fieldVal.Kind() == reflect.Map || fieldVal.Kind() == reflect.Struct) {
			// Handle nested map/struct - recursively create PrettyData
			parser := api.NewStructParser()

			// Create schema for nested structure
			var nestedSchema *api.PrettyObject
			if len(field.Fields) > 0 {
				nestedSchema = &api.PrettyObject{Fields: field.Fields}
			} else {
				// Auto-generate schema
				if fieldVal.Kind() == reflect.Struct {
					nestedSchema, _ = parser.ParseStructSchema(fieldVal)
				} else if fieldVal.Kind() == reflect.Map {
					nestedSchema = &api.PrettyObject{Fields: []api.PrettyField{}}
					// Sort keys for consistent ordering
					keys := fieldVal.MapKeys()
					sort.Slice(keys, func(i, j int) bool {
						return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
					})

					for _, key := range keys {
						if key.Kind() == reflect.String {
							mapValue := fieldVal.MapIndex(key)
							if mapValue.IsValid() {
								nestedSchema.Fields = append(nestedSchema.Fields, api.PrettyField{
									Name: key.String(),
									Type: api.InferValueType(mapValue.Interface()),
								})
							}
						}
					}
				}
			}

			// Recursively parse
			if nestedSchema != nil {
				nestedData, err := parser.ParseDataWithSchema(fieldVal.Interface(), nestedSchema)
				if err == nil {
					prettyData.Values[field.Name] = api.FieldValue{
						Value: nestedData,
						Field: field,
					}
				}
			}
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

	// Check if this is a slice of TreeNode instances
	if isTreeNodeSlice(val) {
		return true
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

// ToSlice converts variadic any arguments to a slice of type T if all elements implement T.
// It handles:
// - Single slice argument: []T or []any where elements are T
// - Multiple arguments: each implementing T
// - Nested slices: flattens one level if first arg is a slice
func ToSlice[T any](data ...any) ([]T, bool) {
	if len(data) == 0 {
		return nil, false
	}

	var result []T

	// Case 1: Single argument that is already a slice
	if len(data) == 1 {
		val := reflect.ValueOf(data[0])
		if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
			// It's a slice, try to convert each element
			for i := 0; i < val.Len(); i++ {
				elem := val.Index(i)
				if elem.CanInterface() {
					if typed, ok := elem.Interface().(T); ok {
						result = append(result, typed)

					} else {

						return nil, false // Not all elements are T
					}
				} else {

					return nil, false
				}
			}
			return result, len(result) > 0
		}
	}

	// Case 2: Multiple arguments or single non-slice argument
	for _, item := range data {
		// Check if this item is a slice (nested slice case)
		val := reflect.ValueOf(item)
		if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
			// Flatten one level
			for i := 0; i < val.Len(); i++ {
				elem := val.Index(i)
				if elem.CanInterface() {
					if typed, ok := elem.Interface().(T); ok {
						result = append(result, typed)
					} else {
						return nil, false
					}
				} else {
					return nil, false
				}
			}
		} else {
			// Single item
			if typed, ok := item.(T); ok {
				result = append(result, typed)
			} else {
				return nil, false
			}
		}
	}

	return result, len(result) > 0
}

// isTreeNodeSlice checks if ALL elements in a slice implement api.TreeNode
func isTreeNodeSlice(val reflect.Value) bool {
	if val.Len() == 0 {
		return false
	}

	// Check ALL elements, not just the first
	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i)

		// Check for TreeNode interface BEFORE dereferencing
		// This is important because TreeNode methods may be defined on pointer receivers
		if !elem.CanInterface() {
			return false
		}

		// Check if it implements api.TreeNode interface (works for both *T and T)
		if _, ok := elem.Interface().(api.TreeNode); !ok {
			return false // Found an element that doesn't implement TreeNode
		}
	}

	return true // All elements implement TreeNode
}

// convertSliceToTreeData converts a slice to tree-formatted PrettyData
func convertSliceToTreeData(val reflect.Value) (*api.PrettyData, error) {
	// Check if this is a slice of TreeNode instances
	if isTreeNodeSlice(val) {
		// Create a root SimpleTreeNode with all items as children
		rootNode := &api.SimpleTreeNode{
			Label:    "",
			Children: make([]api.TreeNode, val.Len()),
		}

		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i)

			// Don't dereference - keep as pointer type to preserve TreeNode interface
			if elem.CanInterface() {
				if treeNode, ok := elem.Interface().(api.TreeNode); ok {
					rootNode.Children[i] = treeNode
				} else {
					// This should not happen due to prior checks
					return nil, fmt.Errorf("element at index %d does not implement TreeNode", i)
				}
			} else {
				// This should not happen due to prior checks
				return nil, fmt.Errorf("element at index %d cannot interface", i)
			}
		}

		// Return PrettyData with the tree structure
		return &api.PrettyData{
			Schema: &api.PrettyObject{Fields: []api.PrettyField{{
				Name:   "tree",
				Format: "tree",
				Label:  "Tree",
			}}},
			Values: map[string]api.FieldValue{
				"tree": {
					Value: rootNode,
					Field: api.PrettyField{
						Name:   "tree",
						Format: "tree",
						Label:  "Tree",
					},
				},
			},
			Tables:   make(map[string][]api.PrettyDataRow),
			Original: val.Interface(),
		}, nil
	}

	// For other tree structures, delegate to table format for now
	return convertSliceToPrettyData(val)
}

// convertSliceToPrettyData converts a slice/array to PrettyData with a table field
func convertSliceToPrettyData(val reflect.Value) (*api.PrettyData, error) {
	// Store the original interface value
	originalData := val.Interface()

	// Flatten slice of slices before processing
	val = FlattenSlice(val)

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

	// Dereference interface to get underlying concrete type
	if firstElem.Kind() == reflect.Interface && !firstElem.IsNil() {
		firstElem = firstElem.Elem()
	}

	// Recursively dereference all pointer layers until we reach a non-pointer type
	for firstElem.Kind() == reflect.Ptr {
		if firstElem.IsNil() {
			return nil, fmt.Errorf("cannot convert slice with nil pointer element")
		}
		firstElem = firstElem.Elem()
	}

	// Handle slices of structs or maps
	if firstElem.Kind() != reflect.Struct && firstElem.Kind() != reflect.Map {
		return nil, fmt.Errorf("can only convert slice of structs or maps to PrettyData, got slice of %s", firstElem.Kind())
	}

	var tableFields []api.PrettyField
	var err error

	// For slices of maps, extract all distinct keys from all maps
	if firstElem.Kind() == reflect.Map {
		keysSet := make(map[string]bool)
		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i)
			elem, isNil := safeDerefPointer(elem)
			if isNil {
				continue
			}
			if elem.Kind() == reflect.Map {
				for _, key := range elem.MapKeys() {
					if key.Kind() == reflect.String {
						keysSet[key.String()] = true
					}
				}
			}
		}
		// Convert to sorted slice of fields
		keys := make([]string, 0, len(keysSet))
		for k := range keysSet {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			tableFields = append(tableFields, api.PrettyField{
				Name:  k,
				Label: k,
				Type:  "string",
			})
		}
	} else {
		// Get the table schema from the first struct element
		tableFields, err = GetTableFields(firstElem)
		if err != nil {
			return nil, fmt.Errorf("failed to get table fields: %w", err)
		}
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
					TableOptions: api.TableOptions{
						Columns: tableFields,
					},
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
