package formatters

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/commons/logger"
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

// Format formats data and returns formatted output
func (p *PrettyFormatter) Format(data interface{}) (string, error) {
	// Check if this is already parsed PrettyData
	if prettyData, ok := data.(*api.PrettyData); ok {
		return p.FormatPrettyData(prettyData)
	}
	return p.Parse(data)
}

// Parse parses a struct and returns formatted output
func (p *PrettyFormatter) Parse(data interface{}) (string, error) {
	if data == nil {
		return "", nil
	}

	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return p.formatValue(val, api.PrettyField{}), nil
	}

	return p.parseStruct(val)
}

// FormatPrettyData formats PrettyData structure
func (p *PrettyFormatter) FormatPrettyData(data *api.PrettyData) (string, error) {
	if data == nil {
		return "", nil
	}

	var result []string

	// Format regular fields
	for _, field := range data.Schema.Fields {
		if field.Format == api.FormatHide {
			continue
		}

		// Skip table fields - they'll be handled separately
		if field.Format == api.FormatTable {
			continue
		}

		if fieldValue, ok := data.Values[field.Name]; ok {
			// Use the field's label or name
			label := field.Label
			if label == "" {
				label = api.PrettifyFieldName(field.Name)
			}

			// Handle nested map fields - check Format, Type, or presence of NestedFields (for schema mismatches)
			if (field.Format == "map" || field.Type == "map" || fieldValue.NestedFields != nil) && fieldValue.NestedFields != nil {
				// Add the field label first
				result = append(result, label+":")
				// Format nested fields with indentation
				nestedLines := p.formatNestedFields(fieldValue, field, 1)
				result = append(result, nestedLines...)
			} else {
				formatted := p.formatField(label, reflect.ValueOf(fieldValue.Value), field)
				result = append(result, formatted)
			}
		}
	}

	// Format table fields
	for _, field := range data.Schema.Fields {
		if field.Format == api.FormatTable {
			if tableRows, ok := data.Tables[field.Name]; ok && len(tableRows) > 0 {
				// Convert table rows to items
				var items []interface{}
				for _, row := range tableRows {
					// Convert row map to struct-like map for table rendering
					rowMap := make(map[string]interface{})
					for k, v := range row {
						rowMap[k] = v.Value
					}
					items = append(items, rowMap)
				}

				// Render table - check if field definitions are available
				var tableStr string
				var err error
				if len(field.Fields) > 0 {
					tableStr, err = p.renderTableFromData(items, field.Fields)
				} else {
					tableStr, err = p.renderTableFromMaps(items)
				}
				if err == nil {
					result = append(result, tableStr)
				}
			}
		}
	}

	return strings.Join(result, "\n"), nil
}

// formatNestedFields formats nested map fields recursively
func (p *PrettyFormatter) formatNestedFields(fieldValue api.FieldValue, field api.PrettyField, indent int) []string {
	var result []string

	// Check if the value is a map with nested values
	if fieldValue.NestedFields != nil {
		if len(fieldValue.NestedFields) == 0 {
			// Empty map case
			indentStr := strings.Repeat("\t", indent)
			result = append(result, indentStr+"(empty)")
			return result
		}
		nestedMap := fieldValue.NestedFields

		// If schema has field definitions, use those for ordering and metadata
		if len(field.Fields) > 0 {
			// Format each nested field using schema definitions
			for _, nestedField := range field.Fields {
				if nestedValue, ok := nestedMap[nestedField.Name]; ok {
					label := nestedField.Label
					if label == "" {
						label = api.PrettifyFieldName(nestedField.Name)
					}

					// Add indentation with tabs
					indentStr := strings.Repeat("\t", indent)

					// Check if this field has further nesting
					if (nestedField.Format == "map" || nestedField.Type == "map") && nestedValue.NestedFields != nil {
						// Recursive formatting
						result = append(result, indentStr+label+":")
						subLines := p.formatNestedFields(nestedValue, nestedField, indent+1)
						result = append(result, subLines...)
					} else {
						// Format the field value
						formatted := p.formatField(label, reflect.ValueOf(nestedValue.Value), nestedField)
						result = append(result, indentStr+formatted)
					}
				}
			}
		} else {
			// No schema definitions - iterate over all nested fields
			for fieldName, nestedValue := range nestedMap {
				label := api.PrettifyFieldName(fieldName)

				// Add indentation with tabs
				indentStr := strings.Repeat("\t", indent)

				// Check if this field has further nesting
				if nestedValue.NestedFields != nil {
					// Recursive formatting - create a minimal field definition
					nestedField := api.PrettyField{Name: fieldName, Type: "map"}
					result = append(result, indentStr+label+":")
					subLines := p.formatNestedFields(nestedValue, nestedField, indent+1)
					result = append(result, subLines...)
				} else {
					// Format the field value with empty field definition
					formatted := p.formatField(label, reflect.ValueOf(nestedValue.Value), api.PrettyField{})
					result = append(result, indentStr+formatted)
				}
			}
		}
	} else {
		// Fall back to simple formatting
		formatted := p.formatField(field.Label, reflect.ValueOf(fieldValue.Value), field)
		result = append(result, formatted)
	}

	return result
}

// renderTableFromData renders a table from map items using field definitions
func (p *PrettyFormatter) renderTableFromData(items []interface{}, fieldDefs []api.PrettyField) (string, error) {
	if len(items) == 0 {
		return "", nil
	}

	// Get headers from field definitions
	var headers []string
	fieldMap := make(map[string]api.PrettyField)
	for _, fieldDef := range fieldDefs {
		headers = append(headers, fieldDef.Name)
		fieldMap[fieldDef.Name] = fieldDef
	}

	// Create rows
	var rows [][]string

	// Header row
	headerRow := make([]string, len(headers))
	for i, header := range headers {
		style := lipgloss.NewStyle().Bold(true)
		if !p.NoColor {
			style = style.Foreground(p.Theme.Primary)
		}
		headerRow[i] = p.applyStyle(header, style)
	}
	rows = append(rows, headerRow)

	// Data rows
	for _, item := range items {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		row := make([]string, len(headers))
		for i, header := range headers {
			if val, ok := itemMap[header]; ok {
				// Use the field definition for proper formatting
				fieldDef := fieldMap[header]
				row[i] = p.formatValue(reflect.ValueOf(val), fieldDef)
			} else {
				row[i] = ""
			}
		}
		rows = append(rows, row)
	}

	return p.formatTableRows(rows), nil
}

// renderTableFromMaps renders a table from map items
func (p *PrettyFormatter) renderTableFromMaps(items []interface{}) (string, error) {
	if len(items) == 0 {
		return "", nil
	}

	// Get headers from first item
	firstItem, ok := items[0].(map[string]interface{})
	if !ok {
		return p.renderTable(items)
	}

	var headers []string
	for key := range firstItem {
		headers = append(headers, key)
	}
	sort.Strings(headers)

	// Create rows
	var rows [][]string

	// Header row
	headerRow := make([]string, len(headers))
	for i, header := range headers {
		style := lipgloss.NewStyle().Bold(true)
		if !p.NoColor {
			style = style.Foreground(p.Theme.Primary)
		}
		headerRow[i] = p.applyStyle(header, style)
	}
	rows = append(rows, headerRow)

	// Data rows
	for _, item := range items {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		row := make([]string, len(headers))
		for i, header := range headers {
			if val, ok := itemMap[header]; ok {
				// Try to find the field definition to get proper formatting
				var field api.PrettyField
				// This is a simple approach - in a full implementation we'd need to pass field definitions
				if strings.Contains(header, "date") || strings.Contains(header, "time") || strings.Contains(header, "at") {
					field.Format = "date"
				} else if strings.Contains(header, "amount") || strings.Contains(header, "price") {
					field.Format = "currency"
				}
				row[i] = p.formatValue(reflect.ValueOf(val), field)
			} else {
				row[i] = ""
			}
		}
		rows = append(rows, row)
	}

	return p.formatTableRows(rows), nil
}

// parseStruct processes a struct and its tags
func (p *PrettyFormatter) parseStruct(val reflect.Value) (string, error) {
	typ := val.Type()
	var fields []string

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if !fieldVal.CanInterface() {
			continue
		}

		prettyTag := field.Tag.Get("pretty")
		jsonTag := field.Tag.Get("json")

		// Skip hidden fields
		if prettyTag == "hide" || prettyTag == api.FormatHide {
			continue
		}

		fieldName := field.Name
		if jsonTag != "" && jsonTag != "-" {
			if parts := strings.Split(jsonTag, ","); parts[0] != "" {
				fieldName = parts[0]
			}
		}

		prettyField := api.ParsePrettyTagWithName(fieldName, prettyTag)

		// Handle table formatting
		if prettyField.Format == api.FormatTable {
			if fieldVal.Kind() == reflect.Slice {
				tableOutput, err := p.formatTable(fieldVal, prettyField)
				if err != nil {
					return "", err
				}
				fields = append(fields, tableOutput)
				continue
			}
		}

		// Handle tree formatting
		if prettyField.Format == api.FormatTree {
			treeOutput := p.formatAsTree(fieldVal, prettyField)
			if treeOutput != "" {
				fields = append(fields, treeOutput)
			}
			continue
		}

		formatted := p.formatField(fieldName, fieldVal, prettyField)
		fields = append(fields, formatted)
	}

	return strings.Join(fields, "\n"), nil
}

// formatField formats a single field
func (p *PrettyFormatter) formatField(name string, val reflect.Value, field api.PrettyField) string {
	labelStyle := lipgloss.NewStyle().Bold(true)
	if !p.NoColor {
		labelStyle = labelStyle.Foreground(p.Theme.Primary)
	}

	valueStr := p.formatValue(val, field)

	return fmt.Sprintf("%s: %s",
		labelStyle.Render(name),
		valueStr)
}

// formatValue formats a value based on the pretty field configuration
func (p *PrettyFormatter) formatValue(val reflect.Value, field api.PrettyField) string {
	return p.formatValueWithVisited(val, field, make(map[uintptr]bool))
}

// formatValueWithVisited formats a value with circular reference detection
func (p *PrettyFormatter) formatValueWithVisited(val reflect.Value, field api.PrettyField, visited map[uintptr]bool) string {
	// Check for custom render function first
	if field.RenderFunc != nil {
		var value interface{}
		if val.IsValid() {
			value = val.Interface()
		}
		return field.RenderFunc(value, field, p.Theme)
	}

	if !val.IsValid() || (val.Kind() == reflect.Ptr && val.IsNil()) {
		return p.applyStyle("null", lipgloss.NewStyle().Foreground(p.Theme.Muted))
	}

	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	switch field.Format {
	case "currency":
		return p.formatCurrency(val)
	case "date":
		return p.formatDate(val, field.FormatOptions["format"])
	case "float":
		return p.formatFloat(val, field.FormatOptions["digits"])
	case "color":
		return p.formatWithColor(val, field.ColorOptions)
	case api.FormatTree:
		return p.formatAsTree(val, field)
	default:
		return p.formatDefaultWithVisited(val, visited)
	}
}

// formatCurrency formats a value as currency
func (p *PrettyFormatter) formatCurrency(val reflect.Value) string {
	style := lipgloss.NewStyle()
	if !p.NoColor {
		style = style.Foreground(p.Theme.Success)
	}

	switch val.Kind() {
	case reflect.Float32, reflect.Float64:
		return p.applyStyle(fmt.Sprintf("$%.2f", val.Float()), style)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return p.applyStyle(fmt.Sprintf("$%.2f", float64(val.Int())), style)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return p.applyStyle(fmt.Sprintf("$%.2f", float64(val.Uint())), style)
	default:
		return p.formatDefault(val)
	}
}

// formatDate formats a value as date
func (p *PrettyFormatter) formatDate(val reflect.Value, format string) string {
	style := lipgloss.NewStyle()
	if !p.NoColor {
		style = style.Foreground(p.Theme.Info)
	}

	var t time.Time

	switch val.Kind() {
	case reflect.String:
		str := val.String()
		// Try parsing as Unix timestamp
		if timestamp, err := strconv.ParseInt(str, 10, 64); err == nil {
			t = time.Unix(timestamp, 0)
		} else {
			// Try parsing as RFC3339
			parsed, err := time.Parse(time.RFC3339, str)
			if err != nil {
				return str
			}
			t = parsed
		}
	case reflect.Int, reflect.Int64:
		t = time.Unix(val.Int(), 0)
	default:
		if val.Type() == reflect.TypeOf(time.Time{}) {
			t = val.Interface().(time.Time)
		} else {
			return p.formatDefault(val)
		}
	}

	if format == "" {
		format = "2006-01-02 15:04:05"
	}
	return p.applyStyle(t.Format(format), style)
}

// formatFloat formats a float with specified precision
func (p *PrettyFormatter) formatFloat(val reflect.Value, digits string) string {
	precision := 2
	if digits != "" {
		if p, err := strconv.Atoi(digits); err == nil {
			precision = p
		}
	}

	style := lipgloss.NewStyle()
	if !p.NoColor {
		style = style.Foreground(p.Theme.Warning)
	}

	switch val.Kind() {
	case reflect.Float32, reflect.Float64:
		format := fmt.Sprintf("%%.%df", precision)
		return p.applyStyle(fmt.Sprintf(format, val.Float()), style)
	default:
		return p.formatDefault(val)
	}
}

// formatWithColor formats a value with specified color
func (p *PrettyFormatter) formatWithColor(val reflect.Value, colorOptions map[string]string) string {
	str := p.formatDefault(val)

	if p.NoColor {
		return str
	}

	style := lipgloss.NewStyle()
	if fg, ok := colorOptions["fg"]; ok {
		style = style.Foreground(lipgloss.Color(fg))
	}
	if bg, ok := colorOptions["bg"]; ok {
		style = style.Background(lipgloss.Color(bg))
	}

	return style.Render(str)
}

// formatDefault formats a value using default formatting
func (p *PrettyFormatter) formatDefault(val reflect.Value) string {
	return p.formatDefaultWithVisited(val, make(map[uintptr]bool))
}

// formatDefaultWithVisited formats a value using default formatting with circular reference detection
func (p *PrettyFormatter) formatDefaultWithVisited(val reflect.Value, visited map[uintptr]bool) string {
	if !val.IsValid() {
		return "null"
	}

	switch val.Kind() {
	case reflect.Ptr:
		if val.IsNil() {
			return "null"
		}
		return p.formatDefaultWithVisited(val.Elem(), visited)
	case reflect.String:
		return val.String()
	case reflect.Bool:
		if val.Bool() {
			if !p.NoColor {
				return lipgloss.NewStyle().Foreground(p.Theme.Success).Render("true")
			}
			return "true"
		}
		if !p.NoColor {
			return lipgloss.NewStyle().Foreground(p.Theme.Error).Render("false")
		}
		return "false"
	case reflect.Map:
		return p.formatMapWithVisited(val, visited)
	case reflect.Slice, reflect.Array:
		return p.formatSliceWithVisited(val, visited)
	case reflect.Struct:
		return p.formatStructWithVisited(val, visited)
	default:
		return fmt.Sprint(val.Interface())
	}
}

// formatMap formats a map value
func (p *PrettyFormatter) formatMap(val reflect.Value) string {
	return p.formatMapWithVisited(val, make(map[uintptr]bool))
}

// formatMapWithVisited formats a map value with circular reference detection
func (p *PrettyFormatter) formatMapWithVisited(val reflect.Value, visited map[uintptr]bool) string {
	if val.IsNil() || val.Len() == 0 {
		return "map[]"
	}

	// Check for circular references
	if val.CanAddr() {
		addr := val.UnsafeAddr()
		if visited[addr] {
			return "map[<circular>]"
		}
		visited[addr] = true
		defer func() { delete(visited, addr) }()
	}

	var parts []string
	keys := val.MapKeys()

	// Sort keys for consistent output
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
	})

	for _, key := range keys {
		value := val.MapIndex(key)
		formattedValue := p.formatValueWithVisited(value, api.PrettyField{}, visited)
		parts = append(parts, fmt.Sprintf("%v:%s", key.Interface(), formattedValue))
	}

	return fmt.Sprintf("map[%s]", strings.Join(parts, " "))
}

// formatSlice formats a slice value
func (p *PrettyFormatter) formatSlice(val reflect.Value) string {
	return p.formatSliceWithVisited(val, make(map[uintptr]bool))
}

// formatSliceWithVisited formats a slice value with circular reference detection
func (p *PrettyFormatter) formatSliceWithVisited(val reflect.Value, visited map[uintptr]bool) string {
	if val.IsNil() || val.Len() == 0 {
		return "[]"
	}

	// Check for circular references
	if val.CanAddr() {
		addr := val.UnsafeAddr()
		if visited[addr] {
			return "[<circular>]"
		}
		visited[addr] = true
		defer func() { delete(visited, addr) }()
	}

	var parts []string
	for i := 0; i < val.Len(); i++ {
		element := val.Index(i)
		formattedValue := p.formatValueWithVisited(element, api.PrettyField{}, visited)
		parts = append(parts, formattedValue)
	}

	return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
}

// formatStructWithVisited formats a struct value with circular reference detection
func (p *PrettyFormatter) formatStructWithVisited(val reflect.Value, visited map[uintptr]bool) string {
	// Check for circular references
	if val.CanAddr() {
		addr := val.UnsafeAddr()
		if visited[addr] {
			return "{<circular>}"
		}
		visited[addr] = true
		defer func() { delete(visited, addr) }()
	}

	// For structs, format them in a compact inline way to avoid infinite recursion
	// This is different from the full parseStruct which formats each field on separate lines
	typ := val.Type()
	var parts []string

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if !fieldVal.CanInterface() {
			continue
		}

		prettyTag := field.Tag.Get("pretty")
		if prettyTag == "hide" || prettyTag == api.FormatHide {
			continue
		}

		// Format field value with visited tracking
		valueStr := p.formatValueWithVisited(fieldVal, api.PrettyField{}, visited)
		parts = append(parts, fmt.Sprintf("%s:%s", field.Name, valueStr))
	}

	return fmt.Sprintf("{%s}", strings.Join(parts, " "))
}

// applyStyle applies a lipgloss style if colors are enabled
func (p *PrettyFormatter) applyStyle(text string, style lipgloss.Style) string {
	if p.NoColor {
		return text
	}
	return style.Render(text)
}

// formatTable formats a slice as a table
func (p *PrettyFormatter) formatTable(val reflect.Value, field api.PrettyField) (string, error) {
	if val.Kind() != reflect.Slice {
		return "", fmt.Errorf("table format requires a slice")
	}

	if val.Len() == 0 {
		return p.applyStyle("(empty table)", lipgloss.NewStyle().Foreground(p.Theme.Muted)), nil
	}

	// Convert slice to []interface{}
	items := make([]interface{}, val.Len())
	for i := 0; i < val.Len(); i++ {
		items[i] = val.Index(i).Interface()
	}

	// Sort if specified
	if sortField := field.FormatOptions["sort"]; sortField != "" {
		direction := field.FormatOptions["dir"]
		if direction == "" {
			direction = "asc"
		}
		p.sortSlice(items, sortField, direction)
	}

	return p.renderTable(items)
}

// sortSlice sorts a slice of structs by a field
func (p *PrettyFormatter) sortSlice(items []interface{}, fieldName, direction string) {
	sort.Slice(items, func(i, j int) bool {
		valI := p.getFieldValue(items[i], fieldName)
		valJ := p.getFieldValue(items[j], fieldName)

		less := p.compareValues(valI, valJ)
		if direction == "desc" {
			return !less
		}
		return less
	})
}

// getFieldValue gets a field value from a struct using reflection
func (p *PrettyFormatter) getFieldValue(item interface{}, fieldName string) interface{} {
	val := reflect.ValueOf(item)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)

		// Check field name
		if field.Name == fieldName {
			return val.Field(i).Interface()
		}

		// Check json tag
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			if parts := strings.Split(jsonTag, ","); parts[0] == fieldName {
				return val.Field(i).Interface()
			}
		}
	}

	return nil
}

// compareValues compares two values for sorting
func (p *PrettyFormatter) compareValues(a, b interface{}) bool {
	if a == nil && b == nil {
		return false
	}
	if a == nil {
		return true
	}
	if b == nil {
		return false
	}

	valA := reflect.ValueOf(a)
	valB := reflect.ValueOf(b)

	// Handle different numeric types
	if valA.Kind() >= reflect.Int && valA.Kind() <= reflect.Float64 &&
		valB.Kind() >= reflect.Int && valB.Kind() <= reflect.Float64 {
		var floatA, floatB float64

		switch valA.Kind() {
		case reflect.Float32, reflect.Float64:
			floatA = valA.Float()
		default:
			floatA = float64(valA.Int())
		}

		switch valB.Kind() {
		case reflect.Float32, reflect.Float64:
			floatB = valB.Float()
		default:
			floatB = float64(valB.Int())
		}

		return floatA < floatB
	}

	// String comparison
	return fmt.Sprintf("%v", a) < fmt.Sprintf("%v", b)
}

// renderTable renders items as a formatted table
func (p *PrettyFormatter) renderTable(items []interface{}) (string, error) {
	if len(items) == 0 {
		return "", nil
	}

	// Get headers from first item
	headers, err := p.getTableHeaders(items[0])
	if err != nil {
		return "", err
	}

	// Create table
	var rows [][]string

	// Add header row
	headerRow := make([]string, len(headers))
	for i, header := range headers {
		style := lipgloss.NewStyle().Bold(true)
		if !p.NoColor {
			style = style.Foreground(p.Theme.Primary)
		}
		headerRow[i] = p.applyStyle(header, style)
	}
	rows = append(rows, headerRow)

	// Add data rows
	for _, item := range items {
		row, err := p.getTableRow(item, headers)
		if err != nil {
			continue // Skip invalid rows
		}
		rows = append(rows, row)
	}

	return p.formatTableRows(rows), nil
}

// getTableHeaders extracts headers from a struct
func (p *PrettyFormatter) getTableHeaders(item interface{}) ([]string, error) {
	val := reflect.ValueOf(item)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("table items must be structs")
	}

	var headers []string
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)

		// Skip hidden fields
		prettyTag := field.Tag.Get("pretty")
		if prettyTag == "hide" || prettyTag == api.FormatHide {
			continue
		}

		// Get display name
		name := field.Name
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			if parts := strings.Split(jsonTag, ","); parts[0] != "" {
				name = parts[0]
			}
		}

		headers = append(headers, name)
	}

	return headers, nil
}

// getTableRow extracts a row from a struct
func (p *PrettyFormatter) getTableRow(item interface{}, headers []string) ([]string, error) {
	val := reflect.ValueOf(item)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("table items must be structs")
	}

	row := make([]string, len(headers))
	typ := val.Type()

	headerIndex := 0
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		// Skip hidden fields
		prettyTag := field.Tag.Get("pretty")
		if prettyTag == "hide" || prettyTag == api.FormatHide {
			continue
		}

		if headerIndex >= len(headers) {
			break
		}

		// Parse pretty tag for formatting
		prettyField := api.ParsePrettyTagWithName(field.Name, prettyTag)

		// Format the value
		formatted := p.formatValue(fieldVal, prettyField)
		row[headerIndex] = formatted
		headerIndex++
	}

	return row, nil
}

// formatTableRows formats table rows with proper alignment
func (p *PrettyFormatter) formatTableRows(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}

	// Calculate column widths
	colWidths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			// Strip ANSI codes for width calculation
			cellWidth := len(stripAnsi(cell))
			if cellWidth > colWidths[i] {
				colWidths[i] = cellWidth
			}
		}
	}

	// Create table style
	borderStyle := lipgloss.NewStyle()
	if !p.NoColor {
		borderStyle = borderStyle.Foreground(p.Theme.Muted)
	}

	var result strings.Builder

	// Top border
	result.WriteString(p.createTableBorder(colWidths, "┌", "┬", "┐", "─", borderStyle))
	result.WriteString("\n")

	// Header row
	if len(rows) > 0 {
		result.WriteString(p.formatTableRow(rows[0], colWidths, borderStyle))
		result.WriteString("\n")

		// Header separator
		if len(rows) > 1 {
			result.WriteString(p.createTableBorder(colWidths, "├", "┼", "┤", "─", borderStyle))
			result.WriteString("\n")
		}
	}

	// Data rows
	for i := 1; i < len(rows); i++ {
		result.WriteString(p.formatTableRow(rows[i], colWidths, borderStyle))
		result.WriteString("\n")
	}

	// Bottom border
	result.WriteString(p.createTableBorder(colWidths, "└", "┴", "┘", "─", borderStyle))

	return result.String()
}

// formatTableRow formats a single table row
func (p *PrettyFormatter) formatTableRow(row []string, colWidths []int, borderStyle lipgloss.Style) string {
	var result strings.Builder

	result.WriteString(p.applyStyle("│", borderStyle))
	for i, cell := range row {
		padding := colWidths[i] - len(stripAnsi(cell))
		result.WriteString(" ")
		result.WriteString(cell)
		result.WriteString(strings.Repeat(" ", padding))
		result.WriteString(" ")
		result.WriteString(p.applyStyle("│", borderStyle))
	}

	return result.String()
}

// createTableBorder creates a table border line
func (p *PrettyFormatter) createTableBorder(colWidths []int, left, mid, right, fill string, style lipgloss.Style) string {
	var result strings.Builder

	result.WriteString(p.applyStyle(left, style))
	for i, width := range colWidths {
		result.WriteString(p.applyStyle(strings.Repeat(fill, width+2), style))
		if i < len(colWidths)-1 {
			result.WriteString(p.applyStyle(mid, style))
		}
	}
	result.WriteString(p.applyStyle(right, style))

	return result.String()
}

// stripAnsi removes ANSI escape codes for width calculation
func stripAnsi(s string) string {
	// Simple ANSI stripping - in production you might want a more robust solution
	var result strings.Builder
	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}

	return result.String()
}

// formatAsTree formats a value as a tree structure
func (p *PrettyFormatter) formatAsTree(val reflect.Value, field api.PrettyField) string {
	// Create tree formatter
	formatter := NewTreeFormatter(p.Theme, p.NoColor, field.TreeOptions)

	// Convert value to tree node
	var node api.TreeNode

	// Check if value already implements TreeNode
	if val.CanInterface() {
		if treeNode, ok := val.Interface().(api.TreeNode); ok {
			node = treeNode
		} else {
			logger.Debugf("Value does not implement TreeNode: %T", val.Interface())
			// Try to convert to tree node
			node = ConvertToTreeNode(val.Interface())
		}
	} else {
		logger.Debugf("Value is not interface{}: %T", val.Interface())
	}

	if node == nil {
		logger.Debugf("Failed to convert to TreeNode: %v", val)
		return p.formatDefault(val)
	}

	// Format the tree
	return formatter.FormatTreeFromRoot(node)
}
