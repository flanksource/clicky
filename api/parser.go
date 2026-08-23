package api

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SafeDerefPointer safely dereferences a pointer value, returning the
// dereferenced value and whether it was nil.
func SafeDerefPointer(val reflect.Value) (reflect.Value, bool) {
	if val.Kind() != reflect.Ptr {
		return val, false
	}
	if val.IsNil() {
		return reflect.Value{}, true
	}
	return val.Elem(), false
}

// shortTextable returns the PrettyShort() rendering of a field value when it
// implements PrettyShort, or nil otherwise. It is the render hook for the
// `short` pretty-tag. Nil pointers (including a typed nil whose PrettyShort has
// a value receiver) return nil rather than panicking, so a `short`-tagged
// relation that is absent renders as empty.
func shortTextable(val reflect.Value) Textable {
	if !val.IsValid() || !val.CanInterface() {
		return nil
	}
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return nil
	}
	if ps, ok := val.Interface().(PrettyShort); ok {
		return ps.PrettyShort()
	}
	return nil
}

// jsonFieldName returns the JSON field name from a struct field's tag,
// falling back to the struct field name if no valid json tag is present.
func jsonFieldName(f reflect.StructField) string {
	if tag := f.Tag.Get("json"); tag != "" && tag != "-" {
		if name, _, _ := strings.Cut(tag, ","); name != "" {
			return name
		}
	}
	return f.Name
}

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

		// Skip hidden fields
		if prettyTag == FormatHide {
			continue
		}

		prettyField := p.parsePrettyTag(prettyTag)
		prettyField.Name = jsonFieldName(field)
		prettyField.SortKey = field.Tag.Get("sort")
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

// inferType infers the type of a field value, delegating to InferValueType
// after safely handling invalid/nil values.
func (p *StructParser) inferType(val reflect.Value) string {
	if !val.IsValid() {
		return "nil"
	}
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return "nil"
	}
	if val.CanInterface() {
		return InferValueType(val.Interface())
	}
	return "unknown"
}

// parseTableField parses a slice field for table formatting
func (p *StructParser) parseTableField(val reflect.Value, field PrettyField) (PrettyField, error) {
	if val.Len() == 0 {
		field.TableOptions = TableOptions{
			Title:         field.Name,
			Columns:       []PrettyField{},
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

	field.TableOptions = TableOptions{
		Title:         field.Name,
		Columns:       tableFields,
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

		// Skip hidden fields
		if prettyTag == FormatHide {
			continue
		}

		prettyField := p.parsePrettyTag(prettyTag)
		prettyField.Name = jsonFieldName(field)
		prettyField.SortKey = field.Tag.Get("sort")
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

		// Skip hidden fields
		if prettyTag == FormatHide {
			continue
		}

		row[jsonFieldName(field)] = p.ProcessFieldValue(fieldVal)
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
	return schema, nil
}

// ParseDataWithSchema parses data into PrettyData using a predefined schema
func (p *StructParser) ParseDataWithSchema(data interface{}, schema *PrettyObject) (*PrettyData, error) {
	if data == nil || schema == nil {
		return &PrettyData{Schema: schema, TypedValue: TypedValue{}}, nil
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
	}

	list := TypedList{}
	values := TypedMap{}

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

		// `short` tag: render the field via its value's PrettyShort() (a
		// compact self-link) instead of Pretty()/Textable.
		if field.Short {
			if short := shortTextable(fieldVal); short != nil {
				values[field.Name] = TypedValue{Textable: short}
				continue
			}
		}

		// Try TryTypedValue first - handles TableProvider, TreeNode, Textable, etc.
		if fieldVal.CanInterface() {
			if tv := TryTypedValue(fieldVal.Interface()); tv != nil {
				// Preserve FieldMeta from schema if format/compactItems is specified
				if field.Format != "" || field.CompactItems {
					tv.FieldMeta = &FieldMeta{
						Name:         field.Name,
						CompactItems: field.CompactItems,
						Format:       field.Format,
					}
				}
				values[field.Name] = *tv
				continue
			}
		}

		// Check if this is a table field
		if field.Format == FormatTable && (fieldVal.Kind() == reflect.Slice || fieldVal.Kind() == reflect.Array) {
			typedValue := NewTypedValue(p.parseTableData(fieldVal, field))
			// Attach field metadata for rendering hints
			typedValue.FieldMeta = &FieldMeta{
				Name:         field.Name,
				CompactItems: field.CompactItems,
				Format:       field.Format,
			}
			values[field.Name] = typedValue
		} else if field.Format == FormatTree {
			// Use NewTypedValue which handles TreeNode and Pretty interfaces
			values[field.Name] = NewTypedValue(fieldVal.Interface())
		} else {
			if fieldVal.CanInterface() {
				if typedValue := TryTypedValue(fieldVal.Interface()); typedValue != nil {
					values[field.Name] = *typedValue
					continue
				}
			}

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
				if err != nil {
					return nil, err
				}
				// Store nested maps/structs in values, not list
				// The TypedValue will contain the nested TypedMap
				if nestedPrettyData.TypedMap != nil {
					values[field.Name] = TypedValue{TypedMap: nestedPrettyData.TypedMap}
				} else {
					values[field.Name] = NewTypedValue(nestedPrettyData)
				}

			} else {
				// Apply field schema transformation if field has type/format specified
				if field.Type != "" && field.Format != "" {
					// Parse the raw value using the field schema
					fieldValue, err := field.Parse(fieldVal.Interface())
					if err != nil {
						return nil, err
					}
					// Convert FieldValue to TypedValue
					if fieldValue.TimeValue != nil {
						values[field.Name] = TypedValue{Textable: Human(*fieldValue.TimeValue)}
					} else {
						values[field.Name] = p.ProcessFieldValue(fieldVal)
					}
				} else {
					// Use ProcessFieldValue to handle pointers and structs
					processedValue := p.ProcessFieldValue(fieldVal)
					values[field.Name] = processedValue
				}
			}
		}
	}

	// Assign the populated values and list to result
	if len(values) > 0 {
		result.TypedMap = &values
	}
	if len(list) > 0 {
		result.TypedList = &list
	}

	return result, nil
}

// parseTableData parses slice data into table rows
func (p *StructParser) parseTableData(val reflect.Value, field PrettyField) TextTable {
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return TextTable{}
	}

	tt := TextTable{}
	tt.Columns = field.TableOptions.Columns

	for _, tableField := range field.TableOptions.Columns {
		// Use Label if provided, otherwise prettify the Name
		headerLabel := tableField.Label
		if headerLabel == "" {
			headerLabel = tableField.prettifyFieldName(tableField.Name)
		}
		tt.Headers = append(tt.Headers, Text{Content: headerLabel})
		tt.FieldNames = append(tt.FieldNames, tableField.Name)
	}

	for i := 0; i < val.Len(); i++ {
		item := val.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}
		if item.Kind() == reflect.Interface && !item.IsNil() {
			item = item.Elem()
		}

		row := TableRow{}

		// Parse each field in the table
		for _, tableField := range field.TableOptions.Columns {
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
				// `short` tag: render the cell via its value's PrettyShort().
				if tableField.Short {
					if short := shortTextable(fieldVal); short != nil {
						row[tableField.Name] = TypedValue{Textable: short}
						continue
					}
				}
				// Use tableField.Parse to apply type-specific formatting (dates, currency, etc.)
				fieldValue, err := tableField.Parse(fieldVal.Interface())
				if err != nil {
					// Fall back to ProcessFieldValue if parsing fails
					row[tableField.Name] = p.ProcessFieldValue(fieldVal)
				} else if fieldValue.Text != nil {
					row[tableField.Name] = TypedValue{Textable: fieldValue.Text}
				} else {
					row[tableField.Name] = p.ProcessFieldValue(fieldVal)
				}
			}
		}

		tt.Rows = append(tt.Rows, row)
	}

	return tt
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
		if field.Name == fieldName || jsonFieldName(field) == fieldName {
			return val.Field(i)
		}
	}

	// Return zero value if not found
	return reflect.Value{}
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
		prettyField.SortKey = field.Tag.Get("sort")

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
						prettyField.TableOptions = TableOptions{
							Columns: tableFields,
						}
					}
				} else if firstElem.Kind() == reflect.Map {
					// Handle slices of maps - extract map keys as table columns
					tableFields := p.GetTableFieldsFromMap(firstElem)
					prettyField.Fields = tableFields
					prettyField.TableOptions = TableOptions{
						Columns: tableFields,
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
						prettyField.TableOptions = TableOptions{
							Columns: tableFields,
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

		prettyField := ParsePrettyTagWithName(jsonFieldName(field), prettyTag)
		prettyField.SortKey = field.Tag.Get("sort")
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
				row[key.String()] = p.ProcessFieldValue(mapValue)
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
			row[key] = NewTypedValue(text)

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
				row[key.String()] = p.ProcessFieldValue(mapValue)
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

		fieldVal := val.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && strings.Contains(jsonTag, "omitempty") && IsEmpty(fieldVal) {
			continue
		}
		row[jsonFieldName(field)] = p.ProcessFieldValue(fieldVal)
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
		if field.Name == fieldName || jsonFieldName(field) == fieldName {
			return val.Field(i)
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
func (p *StructParser) ProcessFieldValue(fieldVal reflect.Value) TypedValue {
	visited := make(map[uintptr]bool)
	return p.processFieldValueWithVisited(fieldVal, visited)
}

// processFieldValueWithVisited is the internal implementation that tracks visited pointers
func (p *StructParser) processFieldValueWithVisited(fieldVal reflect.Value, visited map[uintptr]bool) TypedValue {
	// Track the original pointer address before dereferencing to detect circular references
	var ptrAddr uintptr
	if fieldVal.Kind() == reflect.Ptr && !fieldVal.IsNil() && fieldVal.Elem().Kind() == reflect.Struct {
		ptrAddr = fieldVal.Pointer()
		// Check if we've already visited this pointer (circular reference detected)
		if visited[ptrAddr] {
			return TypedValue{Textable: Text{Content: "[circular reference]"}, IsCircular: true}
		}
		// Mark this pointer as visited before processing
		visited[ptrAddr] = true
		// Defer cleanup so we can process the same struct in different branches
		defer delete(visited, ptrAddr)
	}

	// Recursively dereference all pointer levels
	for fieldVal.Kind() == reflect.Ptr {
		if fieldVal.IsNil() {
			return TypedValue{Textable: nil}
		}
		fieldVal = fieldVal.Elem()
	}

	// Check if the dereferenced value implements Textable interface (e.g., api.Text)
	if fieldVal.IsValid() && fieldVal.CanInterface() {
		if textable, ok := fieldVal.Interface().(Textable); ok {
			return TypedValue{Textable: textable}
		}
	}

	// Check if the dereferenced value implements Pretty interface
	if fieldVal.IsValid() && fieldVal.CanInterface() {
		if pretty, ok := fieldVal.Interface().(Pretty); ok {
			return TypedValue{Textable: pretty.Pretty()}
		}
	}

	// Check if the value is empty/nil/zero via interfaces — skip rendering
	if fieldVal.CanInterface() {
		iface := fieldVal.Interface()
		if e, ok := iface.(interface{ IsEmpty() bool }); ok && e.IsEmpty() {
			return TypedValue{}
		}
		if e, ok := iface.(interface{ IsNil() bool }); ok && e.IsNil() {
			return TypedValue{}
		}
		if e, ok := iface.(interface{ IsZero() bool }); ok && e.IsZero() {
			return TypedValue{}
		}
	}

	// For types implementing fmt.Stringer (time.Time, uuid.UUID, custom structs),
	// use Human() for known types or String() for others, instead of recursing into
	// unexported fields or falling through to the generic handler.
	if fieldVal.CanInterface() {
		if _, ok := fieldVal.Interface().(fmt.Stringer); ok {
			if IsEmpty(fieldVal) {
				return TypedValue{}
			}
			return TypedValue{Textable: Human(fieldVal.Interface())}
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
		return TypedValue{Textable: Text{}.Append(string(bytes))}
	}

	// Handle slices - recursively process all elements
	if fieldVal.Kind() == reflect.Slice {
		result := TypedList{}
		for i := 0; i < fieldVal.Len(); i++ {
			elem := fieldVal.Index(i)
			// Recursively process each element (handles pointers, structs, Pretty, etc.)
			result = append(result, p.processFieldValueWithVisited(elem, visited))
		}
		return TypedValue{TypedList: &result}
	}

	// Handle maps - recursively process all values
	if fieldVal.Kind() == reflect.Map {
		result := make(TypedMap)
		iter := fieldVal.MapRange()
		for iter.Next() {
			k := iter.Key()
			v := iter.Value()

			keyStr := fmt.Sprintf("%v", k.Interface())
			// Recursively process each value (handles pointers, Pretty, etc.)
			result[keyStr] = p.processFieldValueWithVisited(v, visited)
		}
		return TypedValue{TypedMap: &result}
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

		result := make(TypedMap)
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
				if IsEmpty(fVal) {
					continue
				}
			}

			// Recursively process the field value
			result[jsonFieldName(field)] = p.processFieldValueWithVisited(fVal, visited)
		}
		return TypedValue{TypedMap: &result}
	}

	// Return the interface value for primitives (string, int, float, bool, etc.)
	if fieldVal.IsValid() {
		return TypedValue{Textable: Text{}.Append(fieldVal.Interface())}
	}

	return TypedValue{}
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
			if fieldVal.IsValid() && !IsEmpty(fieldVal) {
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
			if fieldVal.IsValid() && !IsEmpty(fieldVal) {
				return fieldVal
			}
		}
	}

	// Fall back to the field name
	return p.getFieldValueByName(val, field.Name)
}

// IsEmpty checks if a value is considered empty.
// It accepts any type: nil, common Go types get fast-path checks,
// and everything else falls back to reflection.
func IsEmpty(v any) bool {
	if v == nil {
		return true
	}

	// Handle reflect.Value passed directly by internal callers
	if rv, ok := v.(reflect.Value); ok {
		if !rv.IsValid() {
			return true
		}
		if rv.CanInterface() {
			return IsEmpty(rv.Interface())
		}
		return isEmptyReflect(rv)
	}

	// Guard: nil pointers boxed in an interface satisfy method-set checks
	// (e.g. (*time.Time)(nil) matches interface{ IsZero() bool }) but will
	// panic when the method is called. Detect and short-circuit.
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Ptr && rv.IsNil() {
		return true
	}

	// Fast-path type switch for common types
	switch val := v.(type) {
	case string:
		return val == ""
	case bool:
		return !val
	case int:
		return val == 0
	case int8:
		return val == 0
	case int16:
		return val == 0
	case int32:
		return val == 0
	case int64:
		return val == 0
	case uint:
		return val == 0
	case uint8:
		return val == 0
	case uint16:
		return val == 0
	case uint32:
		return val == 0
	case uint64:
		return val == 0
	case float32:
		return val == 0
	case float64:
		return val == 0
	case time.Time:
		return val.IsZero()
	case time.Duration:
		return val == 0
	case interface{ IsEmpty() bool }:
		return val.IsEmpty()
	case interface{ IsNil() bool }:
		return val.IsNil()
	case interface{ IsZero() bool }:
		return val.IsZero()
	}

	return isEmptyReflect(reflect.ValueOf(v))
}

// isEmptyReflect is the reflection fallback for IsEmpty.
func isEmptyReflect(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Map, reflect.Chan:
		return v.Len() == 0
	case reflect.Interface:
		if v.IsNil() {
			return true
		}
		if v.Elem().CanInterface() {
			return IsEmpty(v.Elem().Interface())
		}
		return isEmptyReflect(v.Elem())
	case reflect.Ptr:
		if v.IsNil() {
			return true
		}
		return isEmptyReflect(v.Elem())
	default:
		return v.IsZero()
	}
}
