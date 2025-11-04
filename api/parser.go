package api

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/flanksource/commons/logger"
	"gopkg.in/yaml.v3"
)

// StructParser handles parsing of structs into PrettyObject
type StructParser struct{}

// NewStructParser creates a new struct parser
func NewStructParser() *StructParser {
	return &StructParser{}
}

// Parse takes a struct and returns a PrettyObject
func (p *StructParser) Parse(data interface{}) (*PrettyObject, error) {
	if data == nil {
		return &PrettyObject{Fields: []PrettyField{}}, nil
	}

	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("data must be a struct, got %T", data)
	}

	return p.parseStruct(val)
}

// parseStruct processes a struct and its tags
func (p *StructParser) parseStruct(val reflect.Value) (*PrettyObject, error) {
	typ := val.Type()
	var fields []PrettyField

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if !fieldVal.CanInterface() {
			continue
		}

		prettyTag := field.Tag.Get("pretty")
		jsonTag := field.Tag.Get("json")

		// Skip hidden fields
		if prettyTag == FormatHide {
			continue
		}

		fieldName := field.Name
		if jsonTag != "" && jsonTag != "-" {
			if parts := strings.Split(jsonTag, ","); parts[0] != "" {
				fieldName = parts[0]
			}
		}

		prettyField := p.parsePrettyTag(prettyTag)
		prettyField.Name = fieldName
		prettyField.Type = p.inferType(fieldVal)

		// Handle table formatting for slices
		if prettyField.Format == FormatTable && fieldVal.Kind() == reflect.Slice {
			tableField, err := p.parseTableField(fieldVal, prettyField)
			if err != nil {
				return nil, err
			}
			fields = append(fields, tableField)
			continue
		}

		fields = append(fields, prettyField)
	}

	return &PrettyObject{Fields: fields}, nil
}

// parsePrettyTag parses the pretty tag into a PrettyField
func (p *StructParser) parsePrettyTag(tag string) PrettyField {
	return ParsePrettyTag(tag)
}

// inferType infers the type of a field value
func (p *StructParser) inferType(val reflect.Value) string {
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return "nil"
	}
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "int"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "uint"
	case reflect.Float32, reflect.Float64:
		return "float"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map:
		return "map"
	case reflect.Struct:
		// Check if it's a time.Time
		if val.Type() == reflect.TypeOf(time.Time{}) {
			return "date"
		}
		// Check if it's a time.Duration
		if val.Type() == reflect.TypeOf(time.Duration(0)) {
			return "duration"
		}
		return "struct"
	case reflect.Interface:
		if val.IsNil() {
			return "nil"
		}
		return p.inferType(val.Elem())
	default:
		return "unknown"
	}
}

// parseTableField parses a slice field for table formatting
func (p *StructParser) parseTableField(val reflect.Value, field PrettyField) (PrettyField, error) {
	if val.Len() == 0 {
		field.TableOptions = PrettyTable{
			Title:         field.Name,
			Fields:        []PrettyField{},
			Rows:          []map[string]interface{}{},
			SortField:     field.FormatOptions["sort"],
			SortDirection: field.FormatOptions["dir"],
			HeaderStyle:   field.TableOptions.HeaderStyle,
			RowStyle:      field.TableOptions.RowStyle,
		}
		return field, nil
	}

	// Get the first item to determine the structure
	firstItem := val.Index(0)
	if firstItem.Kind() == reflect.Ptr {
		firstItem = firstItem.Elem()
	}

	if firstItem.Kind() != reflect.Struct {
		return field, fmt.Errorf("table items must be structs")
	}

	// Parse the structure of table items
	tableFields, err := p.getTableFields(firstItem)
	if err != nil {
		return field, err
	}

	// Convert all items to rows
	rows := make([]map[string]interface{}, val.Len())
	for i := 0; i < val.Len(); i++ {
		item := val.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}

		row, err := p.structToRow(item)
		if err != nil {
			return field, err
		}
		rows[i] = row
	}

	field.TableOptions = PrettyTable{
		Title:         field.Name,
		Fields:        tableFields,
		Rows:          rows,
		SortField:     field.FormatOptions["sort"],
		SortDirection: field.FormatOptions["dir"],
		HeaderStyle:   field.TableOptions.HeaderStyle,
		RowStyle:      field.TableOptions.RowStyle,
	}

	return field, nil
}

// getTableFields extracts field definitions from a struct for table headers
func (p *StructParser) getTableFields(val reflect.Value) ([]PrettyField, error) {
	typ := val.Type()
	var fields []PrettyField

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if !fieldVal.CanInterface() {
			continue
		}

		prettyTag := field.Tag.Get("pretty")
		jsonTag := field.Tag.Get("json")

		// Skip hidden fields
		if prettyTag == FormatHide {
			continue
		}

		fieldName := field.Name
		if jsonTag != "" && jsonTag != "-" {
			if parts := strings.Split(jsonTag, ","); parts[0] != "" {
				fieldName = parts[0]
			}
		}

		prettyField := p.parsePrettyTag(prettyTag)
		prettyField.Name = fieldName
		prettyField.Type = p.inferType(fieldVal)

		fields = append(fields, prettyField)
	}

	return fields, nil
}

// structToRow converts a struct to a map for table row
func (p *StructParser) structToRow(val reflect.Value) (map[string]interface{}, error) {
	typ := val.Type()
	row := make(map[string]interface{})

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if !fieldVal.CanInterface() {
			continue
		}

		prettyTag := field.Tag.Get("pretty")
		jsonTag := field.Tag.Get("json")

		// Skip hidden fields
		if prettyTag == FormatHide {
			continue
		}

		fieldName := field.Name
		if jsonTag != "" && jsonTag != "-" {
			if parts := strings.Split(jsonTag, ","); parts[0] != "" {
				fieldName = parts[0]
			}
		}

		row[fieldName] = p.ProcessFieldValue(fieldVal)
	}

	return row, nil
}

// ParseValue creates a FieldValue from a raw value and PrettyField definition
func (p *StructParser) ParseValue(value interface{}, field PrettyField) (FieldValue, error) {
	return field.Parse(value)
}

// LoadSchemaFromYAML loads a PrettyObject schema from a YAML file
func (p *StructParser) LoadSchemaFromYAML(filepath string) (*PrettyObject, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	var schema PrettyObject
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse YAML schema: %w", err)
	}

	return &schema, nil
}

// ParseWithSchema parses data using a predefined schema with heuristics
func (p *StructParser) ParseWithSchema(data interface{}, schema *PrettyObject) (*PrettyObject, error) {
	if data == nil || schema == nil {
		return schema, nil
	}

	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Handle both structs and maps
	if val.Kind() != reflect.Struct && val.Kind() != reflect.Map {
		return nil, fmt.Errorf("data must be a struct or map, got %T", data)
	}

	// Apply heuristics to enhance the schema based on actual data
	enhancedSchema := &PrettyObject{
		Fields: make([]PrettyField, len(schema.Fields)),
	}

	copy(enhancedSchema.Fields, schema.Fields)

	// Enhance each field with data-driven heuristics
	for i, field := range enhancedSchema.Fields {
		var fieldVal reflect.Value

		if val.Kind() == reflect.Map {
			fieldVal = p.getMapValueWithAliases(val, field)
		} else {
			fieldVal = p.getFieldValueByNameWithAliases(val, field)
		}

		if fieldVal.IsValid() {
			enhancedField, err := p.enhanceFieldWithHeuristics(field, fieldVal)
			if err != nil {
				return nil, err
			}
			enhancedSchema.Fields[i] = enhancedField
		}
	}

	return enhancedSchema, nil
}

// ParseDataWithSchema parses data into PrettyData using a predefined schema
func (p *StructParser) ParseDataWithSchema(data interface{}, schema *PrettyObject) (*PrettyData, error) {
	if data == nil || schema == nil {
		return &PrettyData{Schema: schema, Values: make(map[string]FieldValue), Tables: make(map[string][]PrettyDataRow)}, nil
	}

	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Handle both structs and maps
	if val.Kind() != reflect.Struct && val.Kind() != reflect.Map {
		return nil, fmt.Errorf("data must be a struct or map, got %T", data)
	}

	result := &PrettyData{
		Schema: schema,
		Values: make(map[string]FieldValue),
		Tables: make(map[string][]PrettyDataRow),
	}

	// Process each field in the schema
	for _, field := range schema.Fields {
		var fieldVal reflect.Value

		if val.Kind() == reflect.Map {
			fieldVal = p.getMapValueWithAliases(val, field)
		} else {
			fieldVal = p.getFieldValueByNameWithAliases(val, field)
		}

		if !fieldVal.IsValid() {
			continue
		}

		// Handle interface{} values
		if fieldVal.Kind() == reflect.Interface && !fieldVal.IsNil() {
			fieldVal = fieldVal.Elem()
		}

		// Check if this is a table field
		if field.Format == FormatTable && (fieldVal.Kind() == reflect.Slice || fieldVal.Kind() == reflect.Array) {
			// Parse table data
			// Check for filter in FormatOptions
			filterExpr := field.FormatOptions["filter"]
			tableRows := p.parseTableData(fieldVal, field, filterExpr)
			result.Tables[field.Name] = tableRows
		} else if field.Format == FormatTree {
			// For tree fields, convert to SimpleTreeNode for consistent formatting
			var treeNode TreeNode
			if tn, ok := fieldVal.Interface().(TreeNode); ok {
				treeNode = TreeNodeToSimple(tn)
			}

			if treeNode != nil {
				// Apply filtering if filter expression provided
				filterExpr := field.FormatOptions["filter"]
				if filterExpr != "" {
					filteredNode, err := FilterTreeNode(treeNode, filterExpr)
					if err != nil {
						logger.Errorf("Failed to apply filter '%s' to tree: %v", filterExpr, err)
					} else {
						treeNode = filteredNode
					}
				}

				// Only store if we have a non-nil tree after filtering
				if treeNode != nil {
					fieldValue, err := field.Parse(treeNode)
					if err == nil {
						result.Values[field.Name] = fieldValue
					}
				}
			}
		} else {
			// Handle nested struct/map fields - recursively create PrettyData
			if (field.Type == "struct" || field.Type == "map") && (fieldVal.Kind() == reflect.Map || fieldVal.Kind() == reflect.Struct) {
				// Create a schema for the nested structure
				var nestedSchema *PrettyObject
				if len(field.Fields) > 0 {
					// Use the fields defined in the parent schema
					nestedSchema = &PrettyObject{Fields: field.Fields}
				} else {
					// Auto-generate schema from the structure
					var err error
					if fieldVal.Kind() == reflect.Struct {
						nestedSchema, err = p.ParseStructSchema(fieldVal)
						if err != nil {
							// Skip if we can't parse the schema
							continue
						}
					} else if fieldVal.Kind() == reflect.Map {
						// For maps without schema, create fields dynamically
						nestedSchema = &PrettyObject{Fields: []PrettyField{}}
						// Sort keys for consistent ordering
						keys := fieldVal.MapKeys()
						sort.Slice(keys, func(i, j int) bool {
							return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
						})

						for _, key := range keys {
							if key.Kind() == reflect.String {
								mapValue := fieldVal.MapIndex(key)
								if mapValue.IsValid() {
									nestedSchema.Fields = append(nestedSchema.Fields, PrettyField{
										Name: key.String(),
										Type: p.inferType(mapValue),
									})
								}
							}
						}
					}
				}

				// Recursively parse the nested structure
				nestedPrettyData, err := p.ParseDataWithSchema(fieldVal.Interface(), nestedSchema)
				if err == nil {
					// Store the nested PrettyData in the FieldValue
					result.Values[field.Name] = FieldValue{
						Field: field,
						Value: nestedPrettyData,
					}
				}
			} else {
				// Parse regular field - use ProcessFieldValue to handle pointers and structs
				processedValue := p.ProcessFieldValue(fieldVal)
				fieldValue, err := field.Parse(processedValue)
				if err != nil {
					// Skip fields that can't be parsed
					continue
				}
				result.Values[field.Name] = fieldValue
			}
		}
	}

	return result, nil
}

// parseTableData parses slice data into table rows
func (p *StructParser) parseTableData(val reflect.Value, field PrettyField, filterExpr string) []PrettyDataRow {
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return nil
	}

	rows := make([]PrettyDataRow, 0, val.Len())

	for i := 0; i < val.Len(); i++ {
		item := val.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}
		if item.Kind() == reflect.Interface && !item.IsNil() {
			item = item.Elem()
		}

		row := make(PrettyDataRow)

		// Parse each field in the table
		for _, tableField := range field.TableOptions.Fields {
			var fieldVal reflect.Value

			if item.Kind() == reflect.Map {
				fieldVal = p.getMapValue(item, tableField.Name)
			} else if item.Kind() == reflect.Struct {
				fieldVal = p.getFieldValueByName(item, tableField.Name)
			} else {
				continue
			}

			if fieldVal.IsValid() {
				if fieldVal.Kind() == reflect.Interface && !fieldVal.IsNil() {
					fieldVal = fieldVal.Elem()
				}
				// Use ProcessFieldValue to handle pointers and structs
				processedValue := p.ProcessFieldValue(fieldVal)
				fieldValue, err := tableField.Parse(processedValue)
				if err == nil {
					row[tableField.Name] = fieldValue
				}
			}
		}

		rows = append(rows, row)
	}

	// Apply filtering if filter expression provided
	if filterExpr != "" {
		filtered, err := FilterTableRows(rows, filterExpr)
		if err != nil {
			// Log error but don't fail - return unfiltered rows
			logger.Errorf("Failed to apply filter '%s': %v", filterExpr, err)
		} else {
			rows = filtered
		}
	}

	return rows
}

// getMapValue gets a value from a map by key name
func (p *StructParser) getMapValue(val reflect.Value, fieldName string) reflect.Value {
	if val.Kind() != reflect.Map {
		return reflect.Value{}
	}

	// Try direct key lookup
	mapVal := val.MapIndex(reflect.ValueOf(fieldName))
	if mapVal.IsValid() {
		return mapVal
	}

	// Try case-insensitive lookup
	for _, key := range val.MapKeys() {
		if key.Kind() == reflect.String {
			if strings.EqualFold(key.String(), fieldName) {
				return val.MapIndex(key)
			}
		}
	}

	return reflect.Value{}
}

// getFieldValueByName gets a field value by name from a struct
func (p *StructParser) getFieldValueByName(val reflect.Value, fieldName string) reflect.Value {
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

// enhanceFieldWithHeuristics applies heuristics to enhance field definition
func (p *StructParser) enhanceFieldWithHeuristics(field PrettyField, val reflect.Value) (PrettyField, error) {
	enhanced := field

	// Auto-detect type if not specified
	if enhanced.Type == "" {
		enhanced.Type = p.inferType(val)
	}

	// Apply format heuristics based on field name and value
	if enhanced.Format == "" {
		enhanced.Format = p.inferFormat(field.Name, val)
	}

	// Apply color heuristics for certain fields
	if enhanced.Color == "" && len(enhanced.ColorOptions) == 0 {
		colorOptions := p.inferColorOptions(field.Name, val)
		if len(colorOptions) > 0 {
			enhanced.ColorOptions = colorOptions
		}
	}

	// For table fields, parse the table structure
	if enhanced.Format == FormatTable && (val.Kind() == reflect.Slice || val.Kind() == reflect.Array) {
		tableField, err := p.parseTableField(val, enhanced)
		if err != nil {
			return enhanced, err
		}
		enhanced = tableField
	}

	return enhanced, nil
}

// inferFormat applies heuristics to determine the best format for a field
func (p *StructParser) inferFormat(fieldName string, val reflect.Value) string {
	fieldNameLower := strings.ToLower(fieldName)

	// Date/time patterns
	if strings.Contains(fieldNameLower, "date") || strings.Contains(fieldNameLower, "time") ||
		strings.Contains(fieldNameLower, "created") || strings.Contains(fieldNameLower, "updated") {
		return "date"
	}

	// Currency patterns
	if strings.Contains(fieldNameLower, "price") || strings.Contains(fieldNameLower, "cost") ||
		strings.Contains(fieldNameLower, "amount") || strings.Contains(fieldNameLower, "total") ||
		strings.Contains(fieldNameLower, "fee") || strings.Contains(fieldNameLower, "charge") {
		return "currency"
	}

	// Table patterns
	if (val.Kind() == reflect.Slice || val.Kind() == reflect.Array) &&
		(strings.Contains(fieldNameLower, "item") || strings.Contains(fieldNameLower, "list") ||
			strings.Contains(fieldNameLower, "entries") || strings.Contains(fieldNameLower, "records")) {
		return FormatTable
	}

	// Float patterns
	if val.Kind() == reflect.Float32 || val.Kind() == reflect.Float64 {
		if strings.Contains(fieldNameLower, "percent") || strings.Contains(fieldNameLower, "rate") {
			return "float"
		}
	}

	return ""
}

// inferColorOptions applies heuristics to determine color coding for fields
func (p *StructParser) inferColorOptions(fieldName string, val reflect.Value) map[string]string {
	fieldNameLower := strings.ToLower(fieldName)
	colorOptions := make(map[string]string)

	// Status field color patterns
	if strings.Contains(fieldNameLower, "status") {
		colorOptions[ColorGreen] = "completed"
		colorOptions[ColorGreen] = "success"
		colorOptions[ColorGreen] = "active"
		colorOptions["yellow"] = "pending"
		colorOptions["yellow"] = "processing"
		colorOptions["red"] = "failed"
		colorOptions["red"] = "canceled"
		colorOptions["red"] = "error"
	}

	// Priority field color patterns
	if strings.Contains(fieldNameLower, "priority") {
		colorOptions["red"] = "high"
		colorOptions["yellow"] = "medium"
		colorOptions[ColorGreen] = "low"
	}

	// Level field color patterns
	if strings.Contains(fieldNameLower, "level") {
		colorOptions["red"] = "critical"
		colorOptions["red"] = "error"
		colorOptions["yellow"] = "warning"
		colorOptions["blue"] = "info"
		colorOptions[ColorGreen] = "debug"
	}

	// Numeric value color patterns
	if val.Kind() >= reflect.Int && val.Kind() <= reflect.Float64 {
		if strings.Contains(fieldNameLower, "score") || strings.Contains(fieldNameLower, "rating") {
			colorOptions[ColorGreen] = ">=80"
			colorOptions["yellow"] = ">=60"
			colorOptions["red"] = "<60"
		}
	}

	return colorOptions
}

// ParseStructSchema creates a PrettyObject schema from struct tags
func (p *StructParser) ParseStructSchema(val reflect.Value) (*PrettyObject, error) {
	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %s", val.Kind())
	}

	typ := val.Type()
	obj := &PrettyObject{
		Fields: []PrettyField{},
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Parse pretty tag
		prettyTag := field.Tag.Get("pretty")
		if prettyTag == "-" || prettyTag == FormatHide || prettyTag == "hide" {
			continue
		}

		prettyField := ParsePrettyTagWithName(field.Name, prettyTag)

		// Check if it's a table field (slice/array of structs)
		fieldVal := val.Field(i)

		// Infer Type if not already set in the tag
		if prettyField.Type == "" {
			prettyField.Type = p.inferType(fieldVal)
		}

		if strings.Contains(prettyTag, "table") && (fieldVal.Kind() == reflect.Slice || fieldVal.Kind() == reflect.Array) {
			prettyField.Format = FormatTable
			// Parse table schema from first element if available
			if fieldVal.Len() > 0 {
				firstElem := fieldVal.Index(0)
				if firstElem.Kind() == reflect.Ptr {
					firstElem = firstElem.Elem()
				}
				if firstElem.Kind() == reflect.Interface && !firstElem.IsNil() {
					firstElem = firstElem.Elem()
				}
				if firstElem.Kind() == reflect.Struct {
					tableFields, err := p.GetTableFields(firstElem)
					if err == nil {
						prettyField.Fields = tableFields
						prettyField.TableOptions = PrettyTable{
							Fields: tableFields,
						}
					}
				} else if firstElem.Kind() == reflect.Map {
					// Handle slices of maps - extract map keys as table columns
					tableFields := p.GetTableFieldsFromMap(firstElem)
					prettyField.Fields = tableFields
					prettyField.TableOptions = PrettyTable{
						Fields: tableFields,
					}
				}
			}
		} else if prettyField.Format != FormatTable && (fieldVal.Kind() == reflect.Slice || fieldVal.Kind() == reflect.Array) {
			// Auto-detect slices of maps or structs as tables (even without explicit "table" tag)
			if fieldVal.Len() > 0 {
				firstElem := fieldVal.Index(0)
				if firstElem.Kind() == reflect.Ptr {
					firstElem = firstElem.Elem()
				}
				if firstElem.Kind() == reflect.Interface && !firstElem.IsNil() {
					firstElem = firstElem.Elem()
				}
				if firstElem.Kind() == reflect.Map || firstElem.Kind() == reflect.Struct {
					prettyField.Format = FormatTable
					var tableFields []PrettyField
					if firstElem.Kind() == reflect.Map {
						tableFields = p.GetTableFieldsFromMap(firstElem)
					} else {
						var err error
						tableFields, err = p.GetTableFields(firstElem)
						if err != nil {
							tableFields = nil
						}
					}
					if len(tableFields) > 0 {
						prettyField.Fields = tableFields
						prettyField.TableOptions = PrettyTable{
							Fields: tableFields,
						}
					}
				}
			}
		}

		obj.Fields = append(obj.Fields, prettyField)
	}

	return obj, nil
}

// GetTableFields extracts fields from a struct for table formatting
func (p *StructParser) GetTableFields(val reflect.Value) ([]PrettyField, error) {
	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct for table row, got %s", val.Kind())
	}

	typ := val.Type()
	var fields []PrettyField

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Parse pretty tag
		prettyTag := field.Tag.Get("pretty")
		if prettyTag == "-" || prettyTag == FormatHide || prettyTag == "hide" {
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

		prettyField := ParsePrettyTagWithName(fieldName, prettyTag)
		fields = append(fields, prettyField)
	}

	return fields, nil
}

// GetTableFieldsFromMap extracts fields from a map for table formatting
func (p *StructParser) GetTableFieldsFromMap(val reflect.Value) []PrettyField {
	if val.Kind() != reflect.Map {
		return nil
	}

	var fields []PrettyField
	keys := val.MapKeys()

	// Sort keys for consistent ordering
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
	})

	for _, key := range keys {
		if key.Kind() == reflect.String {
			keyStr := key.String()
			// Create a PrettyField for each map key
			fields = append(fields, PrettyField{
				Name:  keyStr,
				Label: keyStr,
				Type:  "string", // Default type, could be enhanced by inspecting values
			})
		}
	}

	return fields
}

// StructToRowWithOptions converts a struct to a PrettyDataRow, checking for PrettyRow interface first
func (p *StructParser) StructToRowWithOptions(val reflect.Value, opts interface{}) (PrettyDataRow, error) {
	// Dereference pointer if needed
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, fmt.Errorf("cannot convert nil pointer to row")
		}
		val = val.Elem()
	}

	// Handle maps as rows
	if val.Kind() == reflect.Map {
		row := make(PrettyDataRow)

		// Sort keys for consistent ordering
		keys := val.MapKeys()
		sort.Slice(keys, func(i, j int) bool {
			return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
		})

		for _, key := range keys {
			if key.Kind() == reflect.String {
				mapValue := val.MapIndex(key)
				processedValue := p.ProcessFieldValue(mapValue)
				row[key.String()] = FieldValue{
					Value: processedValue,
					Field: PrettyField{
						Name: key.String(),
						Type: "string",
					},
				}
			}
		}
		return row, nil
	}

	var valInterface any

	if val.CanInterface() {
		valInterface = val.Interface()
	} else {
		return nil, fmt.Errorf("cannot interface struct of kind=%s type=%s", val.Kind(), val.Type().Name())
	}

	// structType := val.Type()
	//logger.V(4).Infof("Processing struct type=%s/%T kind=%s  ", structType.Name(), valInterface, val.Kind())

	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct or map, got %s", val.Kind())
	}

	if prettyRowInterface, ok := valInterface.(PrettyRow); ok {
		// logger.V(5).Infof("Struct %s implements PrettyRow interface - using custom implementation", structType.Name())

		// Use the custom PrettyRow implementation
		prettyRowMap := prettyRowInterface.PrettyRow(opts)
		// logger.V(4).Infof("PrettyRow() returned %d columns for struct %s", len(prettyRowMap), structType.Name())

		// Convert map[string]Text to PrettyDataRow
		row := make(PrettyDataRow)
		for key, text := range prettyRowMap {
			row[key] = FieldValue{
				Value: text.Content,
				Text:  &text,
				Field: PrettyField{Name: key},
			}
		}
		return row, nil
	}
	return p.StructToRow(val)
}

// StructToRow converts a struct to a PrettyDataRow
func (p *StructParser) StructToRow(val reflect.Value) (PrettyDataRow, error) {
	// Dereference pointer if needed
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, fmt.Errorf("cannot convert nil pointer to row")
		}
		val = val.Elem()
	}

	// Handle maps as rows
	if val.Kind() == reflect.Map {
		row := make(PrettyDataRow)

		// Sort keys for consistent ordering
		keys := val.MapKeys()
		sort.Slice(keys, func(i, j int) bool {
			return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
		})

		for _, key := range keys {
			if key.Kind() == reflect.String {
				mapValue := val.MapIndex(key)
				processedValue := p.ProcessFieldValue(mapValue)
				row[key.String()] = FieldValue{
					Value: processedValue,
					Field: PrettyField{
						Name: key.String(),
						Type: "string",
					},
				}
			}
		}
		return row, nil
	}

	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct or map, got %s", val.Kind())
	}

	row := make(PrettyDataRow)
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Skip fields with pretty:"-"
		prettyTag := field.Tag.Get("pretty")
		if prettyTag == "-" || prettyTag == FormatHide || prettyTag == "hide" {
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
		prettyField := ParsePrettyTagWithName(fieldName, prettyTag)

		// Process field value - this normalizes pointers, structs, Pretty implementations, etc.
		processedValue := p.ProcessFieldValue(fieldVal)

		// Create FieldValue
		fv := FieldValue{
			Value: processedValue,
			Field: prettyField,
		}

		// If the processed value is a Text object (from Pretty interface), store it
		if text, ok := processedValue.(Text); ok {
			fv.Text = &text
		}

		row[fieldName] = fv
	}

	return row, nil
}

// GetFieldValue gets a field value by name from a struct
func (p *StructParser) GetFieldValue(val reflect.Value, fieldName string) reflect.Value {
	switch val.Kind() {
	case reflect.Struct:
		return p.getStructFieldValue(val, fieldName)
	case reflect.Map:
		return p.getMapFieldValue(val, fieldName)
	case reflect.Interface:
		// Handle interface{} by checking the underlying value
		if !val.IsNil() {
			return p.GetFieldValue(val.Elem(), fieldName)
		}
		return reflect.Value{}
	default:
		return reflect.Value{}
	}
}

func (p *StructParser) getStructFieldValue(val reflect.Value, fieldName string) reflect.Value {
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

func (p *StructParser) getMapFieldValue(val reflect.Value, fieldName string) reflect.Value {
	// For maps, try to get the value by key
	mapValue := val.MapIndex(reflect.ValueOf(fieldName))
	if mapValue.IsValid() {
		return mapValue
	}

	// Return zero value if not found
	return reflect.Value{}
}

// ProcessFieldValue processes a field value, handling pointers and returning the appropriate value
// This is the central normalization function that converts all values to simple types:
// - Pointers are dereferenced (nil pointers return nil)
// - Pretty implementations are converted to Text objects
// - Structs are converted to maps recursively
// - Slices and maps are processed recursively
// - Circular references are detected and returned as "[circular reference]"
func (p *StructParser) ProcessFieldValue(fieldVal reflect.Value) interface{} {
	visited := make(map[uintptr]bool)
	return p.processFieldValueWithVisited(fieldVal, visited)
}

// processFieldValueWithVisited is the internal implementation that tracks visited pointers
func (p *StructParser) processFieldValueWithVisited(fieldVal reflect.Value, visited map[uintptr]bool) interface{} {
	// Track the original pointer address before dereferencing to detect circular references
	var ptrAddr uintptr
	if fieldVal.Kind() == reflect.Ptr && !fieldVal.IsNil() && fieldVal.Elem().Kind() == reflect.Struct {
		ptrAddr = fieldVal.Pointer()
		// Check if we've already visited this pointer (circular reference detected)
		if visited[ptrAddr] {
			return "[circular reference]"
		}
		// Mark this pointer as visited before processing
		visited[ptrAddr] = true
		// Defer cleanup so we can process the same struct in different branches
		defer delete(visited, ptrAddr)
	}

	// Recursively dereference all pointer levels
	for fieldVal.Kind() == reflect.Ptr {
		if fieldVal.IsNil() {
			return nil
		}
		fieldVal = fieldVal.Elem()
	}

	// Check if the dereferenced value implements Textable interface (e.g., api.Text)
	if fieldVal.IsValid() && fieldVal.CanInterface() {
		if textable, ok := fieldVal.Interface().(Textable); ok {
			return textable
		}
	}

	// Check if the dereferenced value implements Pretty interface
	if fieldVal.IsValid() && fieldVal.CanInterface() {
		if pretty, ok := fieldVal.Interface().(Pretty); ok {
			return pretty.Pretty()
		}
	}

	// Check if value is a byte slice (handles json.RawMessage type aliases)
	// Type aliases don't inherit methods, so we need to check the underlying type
	if fieldVal.Kind() == reflect.Slice && fieldVal.Type().Elem().Kind() == reflect.Uint8 {
		// This is a []byte - convert to string (handles json.RawMessage type aliases)
		bytes := make([]byte, fieldVal.Len())
		for i := 0; i < fieldVal.Len(); i++ {
			bytes[i] = byte(fieldVal.Index(i).Uint())
		}
		return string(bytes)
	}

	// Handle slices - recursively process all elements
	if fieldVal.Kind() == reflect.Slice {
		result := make([]interface{}, fieldVal.Len())
		for i := 0; i < fieldVal.Len(); i++ {
			elem := fieldVal.Index(i)
			// Recursively process each element (handles pointers, structs, Pretty, etc.)
			result[i] = p.processFieldValueWithVisited(elem, visited)
		}
		return result
	}

	// Handle maps - recursively process all values
	if fieldVal.Kind() == reflect.Map {
		result := make(map[string]interface{})
		iter := fieldVal.MapRange()
		for iter.Next() {
			k := iter.Key()
			v := iter.Value()

			keyStr := fmt.Sprintf("%v", k.Interface())
			// Recursively process each value (handles pointers, Pretty, etc.)
			result[keyStr] = p.processFieldValueWithVisited(v, visited)
		}
		return result
	}

	// Handle structs - convert to map with recursively processed fields
	if fieldVal.Kind() == reflect.Struct {
		// Check if this is a FieldValue struct - if so, extract the Value field
		if fieldVal.Type().Name() == "FieldValue" {
			// This is a FieldValue - extract and process its Value field
			valueField := fieldVal.FieldByName("Value")
			if valueField.IsValid() && valueField.CanInterface() {
				return p.processFieldValueWithVisited(valueField, visited)
			}
		}

		result := make(map[string]interface{})
		typ := fieldVal.Type()

		for i := 0; i < fieldVal.NumField(); i++ {
			field := typ.Field(i)
			fVal := fieldVal.Field(i)

			// Skip unexported fields
			if !fVal.CanInterface() {
				continue
			}

			prettyTag := field.Tag.Get("pretty")
			jsonTag := field.Tag.Get("json")

			// Skip hidden fields
			if prettyTag == FormatHide {
				continue
			}

			// Skip omitempty fields if empty
			if jsonTag != "" && strings.Contains(jsonTag, "omitempty") {
				if p.isEmptyValue(fVal) {
					continue
				}
			}

			// Get field name from json tag, fallback to field name
			fieldName := field.Name
			if jsonTag != "" && jsonTag != "-" {
				if parts := strings.Split(jsonTag, ","); parts[0] != "" {
					fieldName = parts[0]
				}
			}

			// Recursively process the field value
			result[fieldName] = p.processFieldValueWithVisited(fVal, visited)
		}
		return result
	}

	// Return the interface value for primitives (string, int, float, bool, etc.)
	if fieldVal.IsValid() {
		return fieldVal.Interface()
	}

	return nil
}

// getMapValueWithAliases tries to get a map value using aliases first, then the field name
func (p *StructParser) getMapValueWithAliases(val reflect.Value, field PrettyField) reflect.Value {
	if val.Kind() != reflect.Map {
		return reflect.Value{}
	}

	// Try aliases first
	if len(field.Aliases) > 0 {
		for _, alias := range field.Aliases {
			fieldVal := p.getMapValue(val, alias)
			if fieldVal.IsValid() && !p.isEmptyValue(fieldVal) {
				return fieldVal
			}
		}
	}

	// Fall back to the field name
	return p.getMapValue(val, field.Name)
}

// getFieldValueByNameWithAliases tries to get a field value using aliases first, then the field name
func (p *StructParser) getFieldValueByNameWithAliases(val reflect.Value, field PrettyField) reflect.Value {
	// Try aliases first
	if len(field.Aliases) > 0 {
		for _, alias := range field.Aliases {
			fieldVal := p.getFieldValueByName(val, alias)
			if fieldVal.IsValid() && !p.isEmptyValue(fieldVal) {
				return fieldVal
			}
		}
	}

	// Fall back to the field name
	return p.getFieldValueByName(val, field.Name)
}

// isEmptyValue checks if a reflect.Value is considered empty
func (p *StructParser) isEmptyValue(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Chan:
		return v.Len() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	default:
		return false
	}
}
