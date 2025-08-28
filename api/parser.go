package api

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

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

		row[fieldName] = fieldVal.Interface()
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
			fieldVal = p.getMapValue(val, field.Name)
		} else {
			fieldVal = p.getFieldValueByName(val, field.Name)
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
			fieldVal = p.getMapValue(val, field.Name)
		} else {
			fieldVal = p.getFieldValueByName(val, field.Name)
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
			tableRows := p.parseTableData(fieldVal, field)
			result.Tables[field.Name] = tableRows
		} else {
			// Handle nested struct/map fields - create nested FieldValues instead of string formatting
			if (field.Type == "struct" || field.Type == "map") && (fieldVal.Kind() == reflect.Map || fieldVal.Kind() == reflect.Struct) {
				// For nested structures, we create a special FieldValue that contains nested fields
				nestedFieldValue := p.createNestedFieldValue(field, fieldVal)
				result.Values[field.Name] = nestedFieldValue
			} else {
				// Parse regular field
				fieldValue, err := field.Parse(fieldVal.Interface())
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
func (p *StructParser) parseTableData(val reflect.Value, field PrettyField) []PrettyDataRow {
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
				fieldValue, err := tableField.Parse(fieldVal.Interface())
				if err == nil {
					row[tableField.Name] = fieldValue
				}
			}
		}

		rows = append(rows, row)
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

// createNestedFieldValue creates a FieldValue with nested fields for struct/map types
func (p *StructParser) createNestedFieldValue(field PrettyField, val reflect.Value) FieldValue {
	nestedFields := make(map[string]FieldValue)

	if val.Kind() == reflect.Map {
		// Handle map as nested fields - combine schema definitions with existing map data
		if len(field.Fields) > 0 {
			// Create a map of schema field definitions for quick lookup
			schemaFields := make(map[string]PrettyField)
			for _, fieldDef := range field.Fields {
				schemaFields[fieldDef.Name] = fieldDef
			}

			// Process all keys in the map
			for _, key := range val.MapKeys() {
				if key.Kind() == reflect.String {
					keyStr := key.String()
					mapValue := val.MapIndex(key)

					if mapValue.IsValid() {
						if mapValue.Kind() == reflect.Interface && !mapValue.IsNil() {
							mapValue = mapValue.Elem()
						}

						var nestedField PrettyField

						// Use schema definition if available, otherwise create a default one
						if schemaDef, exists := schemaFields[keyStr]; exists {
							nestedField = schemaDef

							// Handle date format options
							if nestedField.DateFormat != "" {
								if nestedField.FormatOptions == nil {
									nestedField.FormatOptions = make(map[string]string)
								}
								nestedField.FormatOptions["format"] = nestedField.DateFormat
								if nestedField.Format == "" {
									nestedField.Format = "date"
								}
							}
						} else {
							// Create a simple PrettyField for keys not in schema
							nestedField = PrettyField{
								Name: keyStr,
								Type: InferValueType(mapValue.Interface()),
							}
						}

						// Recursively handle nested maps/structs
						if mapValue.Kind() == reflect.Map || mapValue.Kind() == reflect.Struct {
							nestedFieldValue := p.createNestedFieldValue(nestedField, mapValue)
							nestedFields[keyStr] = nestedFieldValue
						} else {
							// Parse as regular field value with schema formatting
							fieldValue, err := nestedField.Parse(mapValue.Interface())
							if err == nil {
								nestedFields[keyStr] = fieldValue
							}
						}
					}
				}
			}
		} else {
			// Fallback to dynamic field discovery
			for _, key := range val.MapKeys() {
				if key.Kind() == reflect.String {
					keyStr := key.String()
					mapValue := val.MapIndex(key)

					if mapValue.IsValid() {
						if mapValue.Kind() == reflect.Interface && !mapValue.IsNil() {
							mapValue = mapValue.Elem()
						}

						// Create a simple PrettyField for each map key
						nestedField := PrettyField{
							Name: keyStr,
							Type: InferValueType(mapValue.Interface()),
						}

						// Recursively handle nested maps/structs
						if mapValue.Kind() == reflect.Map || mapValue.Kind() == reflect.Struct {
							nestedFieldValue := p.createNestedFieldValue(nestedField, mapValue)
							nestedFields[keyStr] = nestedFieldValue
						} else {
							// Parse as regular field value
							fieldValue, err := nestedField.Parse(mapValue.Interface())
							if err == nil {
								nestedFields[keyStr] = fieldValue
							}
						}
					}
				}
			}
		}
	} else if val.Kind() == reflect.Struct {
		// Handle struct as nested fields
		typ := val.Type()
		for i := 0; i < val.NumField(); i++ {
			structField := typ.Field(i)
			fieldVal := val.Field(i)

			if !fieldVal.CanInterface() {
				continue
			}

			// Get field name (prefer JSON tag)
			fieldName := structField.Name
			jsonTag := structField.Tag.Get("json")
			if jsonTag != "" && jsonTag != "-" {
				if parts := strings.Split(jsonTag, ","); parts[0] != "" {
					fieldName = parts[0]
				}
			}

			// Create PrettyField for struct field
			nestedField := PrettyField{
				Name: fieldName,
				Type: p.inferType(fieldVal),
			}

			// Recursively handle nested maps/structs
			if fieldVal.Kind() == reflect.Map || fieldVal.Kind() == reflect.Struct {
				nestedFieldValue := p.createNestedFieldValue(nestedField, fieldVal)
				nestedFields[fieldName] = nestedFieldValue
			} else {
				// Parse as regular field value
				fieldValue, err := nestedField.Parse(fieldVal.Interface())
				if err == nil {
					nestedFields[fieldName] = fieldValue
				}
			}
		}
	}

	// Create a formatted text representation for nested fields
	var textLines []string
	for fieldName, nestedValue := range nestedFields {
		label := PrettifyFieldName(fieldName)
		formatted := nestedValue.Formatted()
		textLines = append(textLines, fmt.Sprintf("%s: %s", label, formatted))
	}

	textObj := &Text{
		Content: strings.Join(textLines, "\n"),
	}

	return FieldValue{
		Field:        field,
		Value:        val.Interface(),
		NestedFields: nestedFields,
		Text:         textObj,
	}
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
		if strings.Contains(prettyTag, "table") && (fieldVal.Kind() == reflect.Slice || fieldVal.Kind() == reflect.Array) {
			prettyField.Format = FormatTable
			// Parse table schema from first element if available
			if fieldVal.Len() > 0 {
				firstElem := fieldVal.Index(0)
				if firstElem.Kind() == reflect.Ptr {
					firstElem = firstElem.Elem()
				}
				if firstElem.Kind() == reflect.Struct {
					tableFields, err := p.GetTableFields(firstElem)
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

// StructToRow converts a struct to a PrettyDataRow
func (p *StructParser) StructToRow(val reflect.Value) (PrettyDataRow, error) {
	// Dereference pointer if needed
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, fmt.Errorf("cannot convert nil pointer to row")
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %s", val.Kind())
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

		// Process field value
		row[fieldName] = FieldValue{
			Value: p.ProcessFieldValue(fieldVal),
			Field: prettyField,
		}
	}

	return row, nil
}

// GetFieldValue gets a field value by name from a struct
func (p *StructParser) GetFieldValue(val reflect.Value, fieldName string) reflect.Value {
	if val.Kind() != reflect.Struct {
		return reflect.Value{}
	}

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

// ProcessFieldValue processes a field value, handling pointers and returning the appropriate value
func (p *StructParser) ProcessFieldValue(fieldVal reflect.Value) interface{} {
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
