package formatters

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/commons/logger"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
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

	// Use PrettyData.Pretty() to get structured representation of non-table fields
	nonTableOutput := data.Pretty().JoinNewlines().ANSI()
	if nonTableOutput != "" {
		result = append(result, nonTableOutput)
	}

	// Format table fields separately (temporary until Phase 3)
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

	// Build data rows
	var dataRows [][]string
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
		dataRows = append(dataRows, row)
	}

	return p.renderTableWithWriter(headers, dataRows)
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

	// Build data rows
	var dataRows [][]string
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
		dataRows = append(dataRows, row)
	}

	return p.renderTableWithWriter(headers, dataRows)
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
// Deprecated: Use formatFieldLabel with FieldValue.ANSI() instead
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
// Deprecated: Formatting logic should be in FieldValue.Text, use FieldValue.ANSI() instead
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
	case "bytes":
		return p.formatBytes(val)
	case api.FormatTree:
		return p.formatAsTree(val, field)
	default:
		return p.formatDefaultWithVisited(val, visited)
	}
}

func (p *PrettyFormatter) formatBytes(val reflect.Value) string {

	bytes, ok := p.GetInt(val)
	if !ok {
		return ""
	}

	return api.HumanizeBytes(bytes).String()

}

func (p *PrettyFormatter) GetInt(val reflect.Value) (int64, bool) {
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int(), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(val.Uint()), true
	case reflect.Float32, reflect.Float64:
		return int64(val.Float()), true
	case reflect.String:
		if i, err := strconv.ParseInt(val.String(), 10, 64); err == nil {
			return i, true
		}
		return 0, false
	default:
		return 0, false
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
		// Try parsing as Unix timestamp (integer)
		if timestamp, err := strconv.ParseInt(str, 10, 64); err == nil {
			t = time.Unix(timestamp, 0)
		} else if timestamp, err := strconv.ParseFloat(str, 64); err == nil {
			// Try parsing as Unix timestamp (float)
			t = time.Unix(int64(timestamp), 0)
		} else {
			// Try various date string formats
			dateFormats := []string{
				time.RFC3339,
				"2006-01-02 15:04:05",
				"2006-01-02",
				time.RFC3339Nano,
				"2006-01-02T15:04:05",
			}

			var parsed time.Time
			var parseErr error
			for _, format := range dateFormats {
				if parsed, parseErr = time.Parse(format, str); parseErr == nil {
					t = parsed
					break
				}
			}
			if parseErr != nil {
				return str // Return original string if parsing fails
			}
		}
	case reflect.Int, reflect.Int64:
		t = time.Unix(val.Int(), 0)
	case reflect.Float32, reflect.Float64:
		t = time.Unix(int64(val.Float()), 0)
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
		// Check if struct implements Textable interface
		if val.CanInterface() {
			if textable, ok := val.Interface().(api.Textable); ok {
				return textable.ANSI()
			}
		}
		return p.formatStructWithVisited(val, visited)
	default:
		return fmt.Sprint(val.Interface())
	}
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

// formatSliceWithVisited formats a slice value with circular reference detection
func (p *PrettyFormatter) formatSliceWithVisited(val reflect.Value, visited map[uintptr]bool) string {

	if val.Len() == 0 || (val.Kind() == reflect.Slice && val.IsNil()) {
		return ""
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

	// Check if slice elements implement Textable - if so, render one per line
	if val.Len() > 0 {
		firstElem := val.Index(0)
		// Dereference pointers to check the actual type
		checkElem := firstElem
		for checkElem.Kind() == reflect.Ptr && !checkElem.IsNil() {
			checkElem = checkElem.Elem()
		}
		if checkElem.CanInterface() {
			if _, ok := checkElem.Interface().(api.Textable); ok {
				// All elements are Textable - render one per line
				var lines []string
				for i := 0; i < val.Len(); i++ {
					element := val.Index(i)
					// Dereference pointer if needed
					for element.Kind() == reflect.Ptr && !element.IsNil() {
						element = element.Elem()
					}
					if element.CanInterface() {
						if textable, ok := element.Interface().(api.Textable); ok {
							lines = append(lines, textable.ANSI())
						}
					}
				}
				return strings.Join(lines, "\n")
			}
		}
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

		jsonTag := field.Tag.Get("json")

		// Get field name from JSON tag or use field name
		fieldName := field.Name
		if jsonTag != "" && jsonTag != "-" {
			if parts := strings.Split(jsonTag, ","); parts[0] != "" {
				fieldName = parts[0]
			}
		}

		// Check if field value implements Textable interface
		if fieldVal.CanInterface() {
			if textable, ok := fieldVal.Interface().(api.Textable); ok {
				parts = append(parts, fmt.Sprintf("%s:%s", fieldName, textable.ANSI()))
				continue
			}
		}

		// Parse pretty tag to get formatting configuration
		prettyField := api.ParsePrettyTagWithName(fieldName, prettyTag)

		// Format field value with visited tracking and proper formatting
		valueStr := p.formatValueWithVisited(fieldVal, prettyField, visited)
		parts = append(parts, fmt.Sprintf("%s:%s", fieldName, valueStr))
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

// renderTableWithWriter renders a table using the tablewriter library.
// It handles both struct-based and map-based items, extracting headers and formatting rows.
func (p *PrettyFormatter) renderTableWithWriter(headers []string, dataRows [][]string) (string, error) {
	if len(headers) == 0 {
		return "", nil
	}

	// Create buffer to capture table output
	var buf bytes.Buffer

	width := api.GetTerminalWidth()

	// Create tablewriter instance with word wrapping enabled
	// Set reasonable table max width to enable wrapping (this is distributed across columns)
	table := tablewriter.NewTable(&buf,
		tablewriter.WithRowAutoWrap(tw.WrapTruncate),
		tablewriter.WithDebug(true),
		tablewriter.WithHeaderAutoFormat(tw.On),
		tablewriter.WithMaxWidth(width),
	)

	// Set headers
	table.Header(headers)

	// Append data rows
	for _, row := range dataRows {
		// Convert []string to []any for Append method
		rowData := make([]any, len(row))
		for i, cell := range row {
			rowData[i] = cell
		}
		if err := table.Append(rowData); err != nil {
			return "", fmt.Errorf("failed to append row: %w", err)
		}
	}

	// Render the table
	if err := table.Render(); err != nil {
		return "", fmt.Errorf("failed to render table: %w", err)
	}

	logger.Errorf(table.Debug().String())

	return buf.String(), nil
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

	// Build data rows
	var dataRows [][]string
	for _, item := range items {
		row, err := p.getTableRow(item, headers)
		if err != nil {
			continue // Skip invalid rows
		}
		dataRows = append(dataRows, row)
	}

	return p.renderTableWithWriter(headers, dataRows)
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
