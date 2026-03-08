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
			if fieldVal.IsValid() && !api.IsEmpty(fieldVal) {
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

// processFieldValue processes a field value, handling pointers and returning the appropriate value for FieldValue
func processFieldValue(fieldVal reflect.Value) interface{} {
	parser := api.NewStructParser()
	return parser.ProcessFieldValue(fieldVal)
}

// ToPrettyDataWithOptions converts various input types to PrettyData using format options
func ToPrettyDataWithOptions(data interface{}, opts FormatOptions) (*api.PrettyData, error) {
	// Handle nil data at root level
	if data == nil {
		return &api.PrettyData{
			Schema: &api.PrettyObject{Fields: []api.PrettyField{}},

			Original: data,
		}, nil
	}

	// Check if data is already a known typed value (TextTable, TextTree, etc.)
	if v := api.TryTypedValue(data); v != nil {
		return &api.PrettyData{
			Original:   data,
			TypedValue: *v,
		}, nil
	}

	// Check if data implements Pretty interface first
	if pretty, ok := data.(api.Pretty); ok {
		return &api.PrettyData{
			Original:   data,
			TypedValue: api.NewTypedValue(pretty),
		}, nil

	}

	// Get reflect value
	val := reflect.ValueOf(data)

	// Handle nil pointer at root level
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return &api.PrettyData{
			Schema:   &api.PrettyObject{Fields: []api.PrettyField{}},
			Original: data,
		}, nil
	}

	// Check if it's a slice/array
	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		return parseSliceDataWithOptions(val, opts)
	}

	// For single objects (struct or map)
	return parseStructDataWithOptions(val, opts)
}

// parseSliceDataWithOptions handles slice/array data with format options
func parseSliceDataWithOptions(val reflect.Value, opts FormatOptions) (*api.PrettyData, error) {
	// Safely dereference root level pointer
	var isNil bool
	val, isNil = api.SafeDerefPointer(val)
	if isNil {
		return &api.PrettyData{
			Schema: &api.PrettyObject{Fields: []api.PrettyField{}},
		}, nil
	}
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
	var isNil bool
	val, isNil = api.SafeDerefPointer(val)
	if isNil {
		return &api.PrettyData{
			Schema: &api.PrettyObject{Fields: []api.PrettyField{}},
		}, nil
	}

	// Check dereferenced value for Pretty interface
	if val.CanInterface() {
		if p, ok := val.Interface().(api.Pretty); ok {
			return &api.PrettyData{
				Original:   p,
				TypedValue: api.NewTypedValue(p),
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
									tableFields = append(tableFields, api.PrettyField{
										Name:  keyStr,
										Label: keyStr,
										Type:  api.InferValueType(mapVal.Interface()),
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
			Schema: &api.PrettyObject{Fields: []api.PrettyField{}},

			Original: originalData,
		}, nil
	}

	// Get the first element and unwrap pointers/interfaces
	firstElem, err := unwrapElement(val.Index(0))
	if err != nil {
		return nil, err
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
	// For slices of structs, extract fields from struct tags first to get proper labels
	if firstElem.Kind() == reflect.Map {
		keysSet := make(map[string]bool)
		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i)
			elem, isNil := api.SafeDerefPointer(elem)
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
	} else if firstElem.Kind() == reflect.Struct {
		// Check if the element implements PrettyRow before using reflection
		if firstElem.CanInterface() {
			if _, ok := firstElem.Interface().(api.PrettyRow); ok {
				// PrettyRow is implemented - skip reflection, columns will come from PrettyRow output
				// logger.V(4).Infof("Struct implements PrettyRow - skipping reflection-based field extraction")
			} else {
				// No PrettyRow - extract fields from struct tags for proper labels
				var err error
				tableFields, err = GetTableFields(firstElem)
				if err != nil {
					logger.V(4).Infof("Failed to get table fields from struct: %v", err)
				}
			}
		}
	}

	// Statistics for PrettyRow usage
	prettyRowCount := 0
	reflectionCount := 0

	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i)
		elem, isNil := api.SafeDerefPointer(elem)
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
			// Create a slice to store columns with their order values for sorting
			type columnWithOrder struct {
				field      api.PrettyField
				orderValue int
			}
			columnsToSort := make([]columnWithOrder, 0, len(row))

			for columnName, typedValue := range row {
				field := api.PrettyField{
					Name:  columnName,
					Label: columnName,
					Type:  "string", // Default type, could be enhanced
				}

				// Extract order value from the column's style if it's a Text object
				orderValue := 0
				if textable := typedValue.Value(); textable != nil {
					if text, ok := textable.(api.Text); ok {
						orderValue = api.ExtractOrderValue(text.Style)
					}
				}

				columnsToSort = append(columnsToSort, columnWithOrder{
					field:      field,
					orderValue: orderValue,
				})
			}

			// Sort columns by order value (columns without order-X get 0 and appear first)
			sort.SliceStable(columnsToSort, func(i, j int) bool {
				return columnsToSort[i].orderValue < columnsToSort[j].orderValue
			})

			// Extract sorted fields
			for _, col := range columnsToSort {
				tableFields = append(tableFields, col.field)
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

	// Create table with column schema
	table := api.NewTableFromRows(rows)
	table.Columns = tableFields

	// Update headers to use Labels from tableFields (not raw field names)
	if len(tableFields) > 0 {
		table.Headers = make(api.TextList, 0, len(tableFields))
		table.FieldNames = make([]string, 0, len(tableFields))
		for _, field := range tableFields {
			headerLabel := field.Label
			if headerLabel == "" {
				headerLabel = api.PrettifyFieldName(field.Name)
			}
			table.Headers = append(table.Headers, api.Text{Content: headerLabel})
			table.FieldNames = append(table.FieldNames, field.Name)
		}
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

		TypedValue: api.TypedValue{Table: &table},
		Original:   originalData,
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

	return prettyData, nil
}

// TypeOptions controls how data is converted to PrettyData
type TypeOptions struct {
	SkipTable bool
	SkipTree  bool
}

func mergeTypeOptions(opts ...TypeOptions) TypeOptions {
	merged := TypeOptions{}
	for _, opt := range opts {
		merged.SkipTable = merged.SkipTable || opt.SkipTable
		merged.SkipTree = merged.SkipTree || opt.SkipTree
	}
	return merged
}

// ToPrettyData converts various input types to PrettyData
func ToPrettyData(data interface{}, opts ...TypeOptions) (*api.PrettyData, error) {
	opt := mergeTypeOptions(opts...)
	// Handle nil data at root level
	if data == nil {
		return &api.PrettyData{
			Schema:   &api.PrettyObject{Fields: []api.PrettyField{}},
			Original: data,
		}, nil
	}

	if v := api.TryTypedValue(data); v != nil {
		return &api.PrettyData{
			Original:   data,
			TypedValue: *v,
		}, nil
	}

	// Parse the input data
	val := reflect.ValueOf(data)

	// Handle nil pointer at root level
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return &api.PrettyData{
			Schema:   &api.PrettyObject{Fields: []api.PrettyField{}},
			Original: data,
		}, nil
	}

	// Safely dereference root level pointer
	val, _ = api.SafeDerefPointer(val)

	// Check dereferenced value for Pretty interface
	if val.CanInterface() {
		val := val.Interface()
		if v := api.TryTypedValue(val); v != nil {
			return &api.PrettyData{
				Original:   data,
				TypedValue: *v,
			}, nil
		}
	}

	val = FlattenSlice(val)
	// Handle slices/arrays - default to table format unless items have tree structure
	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		if !opt.SkipTree && hasTreeStructure(val) {
			return convertSliceToTreeData(val)
		}
		if !opt.SkipTable {
			return convertSliceToPrettyData(val)
		}
		// Both SkipTable and SkipTree are set - cannot format slice
		return nil, fmt.Errorf("cannot format slice input when both SkipTable and SkipTree are set")
	}

	// Create the schema from struct tags
	schema, err := ParseStructSchema(val)
	if err != nil {
		return nil, fmt.Errorf("failed to parse struct schema: %w", err)
	}

	// Create PrettyData from the schema and values
	prettyData := &api.PrettyData{
		Schema:   schema,
		Original: data,
	}

	values := api.TypedMap{}

	// Process each field
	for _, field := range schema.Fields {
		// Try aliases first, then the field name
		fieldVal := GetFieldValueWithAliases(val, field)
		if !fieldVal.IsValid() {
			continue
		}

		// Try TryTypedValue first - handles TableProvider, TreeNode, Textable, etc.
		if fieldVal.CanInterface() {
			if tv := api.TryTypedValue(fieldVal.Interface()); tv != nil {
				values[field.Name] = *tv
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
				processedElem, isNil := api.SafeDerefPointer(elem)
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
			// Create table with column schema
			table := api.NewTableFromRows(rows)
			table.Columns = field.TableOptions.Columns
			values[field.Name] = api.TypedValue{Table: &table}

		} else if field.Format == api.FormatTree {
			values[field.Name] = api.NewTypedValue(fieldVal.Interface())
		} else if fieldVal.CanInterface() {
			if typedValue := api.TryTypedValue(fieldVal.Interface()); typedValue != nil {
				values[field.Name] = *typedValue
				continue
			}

			if (field.Type == "map" || field.Type == "struct") && (fieldVal.Kind() == reflect.Map || fieldVal.Kind() == reflect.Struct) {
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
					if err != nil {
						return nil, fmt.Errorf("failed to parse nested field %s: %w", field.Name, err)
					} else {
						values[field.Name] = api.NewTypedValue(nestedData)
					}
				}
			} else {
				values[field.Name] = api.NewTypedValue(processFieldValue(fieldVal))
			}
		}
	}
	prettyData.TypedMap = &values

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
	firstElem, _ = api.SafeDerefPointer(firstElem)

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
			TypedValue: *api.TryTypedValue(rootNode),

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
			Schema: &api.PrettyObject{Fields: []api.PrettyField{}},

			Original: originalData,
		}, nil
	}

	// Get the first element and unwrap pointers/interfaces
	firstElem, err := unwrapElement(val.Index(0))
	if err != nil {
		return nil, err
	}

	// Handle slices of structs or maps
	if firstElem.Kind() != reflect.Struct && firstElem.Kind() != reflect.Map {
		return nil, fmt.Errorf("can only convert slice of structs or maps to PrettyData, got slice of %s", firstElem.Kind())
	}

	var tableFields []api.PrettyField

	// For slices of maps, extract all distinct keys from all maps
	if firstElem.Kind() == reflect.Map {
		keysSet := make(map[string]bool)
		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i)
			elem, isNil := api.SafeDerefPointer(elem)
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
		elem, isNil := api.SafeDerefPointer(elem)
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
	// Create table with column schema
	table := api.NewTableFromRows(rows)
	table.Columns = tableFields

	// Update headers to use Labels from tableFields (not raw field names)
	if len(tableFields) > 0 {
		table.Headers = make(api.TextList, 0, len(tableFields))
		table.FieldNames = make([]string, 0, len(tableFields))
		for _, field := range tableFields {
			headerLabel := field.Label
			if headerLabel == "" {
				headerLabel = api.PrettifyFieldName(field.Name)
			}
			table.Headers = append(table.Headers, api.Text{Content: headerLabel})
			table.FieldNames = append(table.FieldNames, field.Name)
		}
	}

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
		TypedValue: api.TypedValue{Table: &table},
		Original:   originalData,
	}, nil
}

// StructToRow converts a struct to a PrettyDataRow
// Deprecated: Use api.StructParser.StructToRow instead
func StructToRow(val reflect.Value) (api.PrettyDataRow, error) {
	parser := api.NewStructParser()
	return parser.StructToRow(val)
}
