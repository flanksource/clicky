package formatters

import (
	"fmt"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/tailwind"
	"reflect"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PrettyFormatter handles formatting of PrettyObject to styled output
type PrettyFormatter struct {
	Theme   api.Theme
	NoColor bool
}

// NewPrettyFormatter creates a new formatter with default theme
func NewPrettyFormatter() *PrettyFormatter {
	return &PrettyFormatter{
		Theme: api.DefaultTheme(),
	}
}

// Format formats data into styled output, accepting any interface{}
func (f *PrettyFormatter) Format(data interface{}) (string, error) {
	// Convert to PrettyData
	prettyData, err := f.ToPrettyData(data)
	if err != nil {
		return "", fmt.Errorf("failed to convert to PrettyData: %w", err)
	}
	
	if prettyData == nil || prettyData.Schema == nil {
		return "", nil
	}

	return f.formatPrettyData(prettyData)
}

// ToPrettyData converts various input types to PrettyData
func (f *PrettyFormatter) ToPrettyData(data interface{}) (*api.PrettyData, error) {
	return ToPrettyData(data)
}

// parseStructSchema creates a PrettyObject schema from struct tags
func (f *PrettyFormatter) parseStructSchema(val reflect.Value) (*api.PrettyObject, error) {
	return ParseStructSchema(val)
}

// parsePrettyTag parses a pretty tag string into a PrettyField
func (f *PrettyFormatter) parsePrettyTag(fieldName string, tag string) api.PrettyField {
	return ParsePrettyTag(fieldName, tag)
}

// getTableFields extracts fields from a struct for table formatting
func (f *PrettyFormatter) getTableFields(val reflect.Value) ([]api.PrettyField, error) {
	return GetTableFields(val)
}

// getFieldValueCaseInsensitive tries to find a field by name with different casing
func (f *PrettyFormatter) getFieldValueCaseInsensitive(val reflect.Value, name string) reflect.Value {
	return GetFieldValueCaseInsensitive(val, name)
}

// structToRow converts a struct to a PrettyDataRow
func (f *PrettyFormatter) structToRow(val reflect.Value) (api.PrettyDataRow, error) {
	return StructToRow(val)
}

// formatStruct formats a struct using the PrettyObject definition
func (f *PrettyFormatter) formatStruct(obj *api.PrettyObject, val reflect.Value) (string, error) {
	var sections []string
	var summaryFields []api.PrettyField
	var tableFields []api.PrettyField

	// First pass: separate table fields from summary fields
	for _, field := range obj.Fields {
		if field.Format == api.FormatTable {
			tableFields = append(tableFields, field)
		} else {
			summaryFields = append(summaryFields, field)
		}
	}

	// Format summary fields first in 2-column layout
	if len(summaryFields) > 0 {
		summaryOutput := f.formatSummaryFields(summaryFields, val)
		sections = append(sections, summaryOutput)
	}

	// Then handle tables
	for _, field := range tableFields {
		fieldVal := f.getFieldValue(val, field.Name)
		if fieldVal.Kind() == reflect.Slice {
			tableOutput, err := f.formatTable(fieldVal, field)
			if err != nil {
				return "", err
			}
			sections = append(sections, tableOutput)
		}
	}

	return strings.Join(sections, "\n"), nil
}

// formatSummaryFields formats non-table fields in a 2-column layout
func (f *PrettyFormatter) formatSummaryFields(fields []api.PrettyField, val reflect.Value) string {
	if len(fields) == 0 {
		return ""
	}

	// Create pairs of fields for 2-column layout
	var rows []string
	for i := 0; i < len(fields); i += 2 {
		leftField := fields[i]
		leftVal := f.getFieldValue(val, leftField.Name)
		leftFormatted := f.formatFieldForSummary(leftField.Name, leftVal, leftField)

		if i+1 < len(fields) {
			// Two columns
			rightField := fields[i+1]
			rightVal := f.getFieldValue(val, rightField.Name)
			rightFormatted := f.formatFieldForSummary(rightField.Name, rightVal, rightField)

			// Create side-by-side layout with wider columns
			leftColumn := lipgloss.NewStyle().Width(50).Render(leftFormatted)
			rightColumn := lipgloss.NewStyle().Width(50).Render(rightFormatted)
			row := lipgloss.JoinHorizontal(lipgloss.Left, leftColumn, rightColumn)
			rows = append(rows, row)
		} else {
			// Single column for odd number of fields
			rows = append(rows, leftFormatted)
		}
	}

	return strings.Join(rows, "\n")
}

// formatFieldForSummary formats a field for the summary section with pretty field names
func (f *PrettyFormatter) formatFieldForSummary(name string, val reflect.Value, field api.PrettyField) string {
	prettyName := f.prettifyFieldName(name)
	valueStr := f.formatValue(val.Interface(), field)

	// Apply label_style if specified, otherwise use default
	var styledLabel string
	if field.LabelStyle != "" {
		styledLabel = f.applyTailwindStyleToText(prettyName, field.LabelStyle)
	} else {
		// Default label style
		labelStyle := lipgloss.NewStyle().Bold(true)
		if !f.NoColor {
			labelStyle = labelStyle.Foreground(f.Theme.Primary)
		}
		styledLabel = f.applyStyle(prettyName, labelStyle)
	}

	return fmt.Sprintf("%s: %s", styledLabel, valueStr)
}

// prettifyFieldName converts field names to readable format
func (f *PrettyFormatter) prettifyFieldName(name string) string {
	return PrettifyFieldName(name)
}

// splitCamelCase splits camelCase strings into words
func (f *PrettyFormatter) splitCamelCase(s string) []string {
	return SplitCamelCase(s)
}

// getFieldValue gets a field value by name from a struct
func (f *PrettyFormatter) getFieldValue(val reflect.Value, fieldName string) reflect.Value {
	return GetFieldValue(val, fieldName)
}

// formatField formats a single field
func (f *PrettyFormatter) formatField(name string, val reflect.Value, field api.PrettyField) string {
	labelStyle := lipgloss.NewStyle().Bold(true)
	if !f.NoColor {
		labelStyle = labelStyle.Foreground(f.Theme.Primary)
	}

	valueStr := f.formatValue(val.Interface(), field)

	return fmt.Sprintf("%s: %s",
		f.applyStyle(name, labelStyle),
		valueStr)
}

// formatValue formats a value based on the pretty field configuration
func (f *PrettyFormatter) formatValue(value interface{}, field api.PrettyField) string {
	if value == nil {
		return f.applyStyle("null", lipgloss.NewStyle().Foreground(f.Theme.Muted))
	}

	val := reflect.ValueOf(value)
	if val.Kind() == reflect.Ptr && val.IsNil() {
		return f.applyStyle("null", lipgloss.NewStyle().Foreground(f.Theme.Muted))
	}
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
		value = val.Interface()
	}

	// Handle nested structs with indentation
	if val.Kind() == reflect.Struct {
		return f.formatNestedStruct(val, 0)
	}

	// Parse value using FieldValue
	fieldValue, err := field.Parse(value)
	if err != nil {
		return f.formatDefault(value)
	}

	// Get formatted string from FieldValue
	formatted := fieldValue.Formatted()

	// Apply style if specified
	if field.Style != "" {
		return f.applyTailwindStyleToText(formatted, field.Style)
	}

	// Apply color styling using FieldValue.Color()
	if color := fieldValue.Color(); color != "" {
		style := f.getColorStyle(color)
		return f.applyStyle(formatted, style)
	}

	// Apply specific styling based on format
	switch field.Format {
	case api.FormatCurrency:
		style := lipgloss.NewStyle()
		if !f.NoColor {
			style = style.Foreground(f.Theme.Success)
		}
		return f.applyStyle(formatted, style)
	case api.FormatDate:
		style := lipgloss.NewStyle()
		if !f.NoColor {
			style = style.Foreground(f.Theme.Info)
		}
		return f.applyStyle(formatted, style)
	case api.FormatFloat:
		style := lipgloss.NewStyle()
		if !f.NoColor {
			style = style.Foreground(f.Theme.Secondary)
		}
		return f.applyStyle(formatted, style)
	default:
		return formatted
	}
}

// formatNestedStruct formats a nested struct with indentation
func (f *PrettyFormatter) formatNestedStruct(val reflect.Value, indentLevel int) string {
	if val.Kind() != reflect.Struct {
		return f.formatDefault(val.Interface())
	}

	typ := val.Type()
	var lines []string
	indent := strings.Repeat("  ", indentLevel)

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if !fieldVal.CanInterface() {
			continue
		}

		// Skip hidden fields
		prettyTag := field.Tag.Get("pretty")
		if strings.Contains(prettyTag, api.FormatHide) {
			continue
		}

		fieldName := field.Name
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			if parts := strings.Split(jsonTag, ","); len(parts) > 0 && parts[0] != "" {
				fieldName = parts[0]
			}
		}

		prettyName := f.prettifyFieldName(fieldName)

		labelStyle := lipgloss.NewStyle().Bold(true)
		if !f.NoColor {
			labelStyle = labelStyle.Foreground(f.Theme.Primary)
		}

		var valueStr string
		if fieldVal.Kind() == reflect.Struct {
			// Recursively format nested structs
			valueStr = "\n" + f.formatNestedStruct(fieldVal, indentLevel+1)
		} else {
			// Format primitive values
			prettyField := api.PrettyField{} // Use default formatting for nested fields
			valueStr = f.formatValue(fieldVal.Interface(), prettyField)
		}

		line := fmt.Sprintf("%s%s: %s",
			indent,
			f.applyStyle(prettyName, labelStyle),
			valueStr)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// formatDefault formats a value using default formatting
func (f *PrettyFormatter) formatDefault(value interface{}) string {
	return fmt.Sprintf("%v", value)
}

// formatTable formats a slice as a table
func (f *PrettyFormatter) formatTable(val reflect.Value, field api.PrettyField) (string, error) {
	if val.Kind() != reflect.Slice {
		return "", fmt.Errorf("table format requires a slice")
	}

	if val.Len() == 0 {
		return f.applyStyle("(empty table)", lipgloss.NewStyle().Foreground(f.Theme.Muted)), nil
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
		f.sortSlice(items, sortField, direction)
	}

	return f.renderTable(items, field.TableOptions.Fields)
}

// sortSlice sorts a slice of structs by a field
func (f *PrettyFormatter) sortSlice(items []interface{}, fieldName string, direction string) {
	sort.Slice(items, func(i, j int) bool {
		valI := f.getStructFieldValue(items[i], fieldName)
		valJ := f.getStructFieldValue(items[j], fieldName)

		less := f.compareValues(valI, valJ)
		if direction == "desc" {
			return !less
		}
		return less
	})
}

// getStructFieldValue gets a field value from a struct using reflection
func (f *PrettyFormatter) getStructFieldValue(item interface{}, fieldName string) interface{} {
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
func (f *PrettyFormatter) compareValues(a, b interface{}) bool {
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
func (f *PrettyFormatter) renderTable(items []interface{}, fields []api.PrettyField) (string, error) {
	if len(items) == 0 {
		return "", nil
	}

	// Get headers
	headers := make([]string, len(fields))
	for i, field := range fields {
		headers[i] = field.Name
	}

	// Create table
	var rows [][]string

	// Add header row
	headerRow := make([]string, len(headers))
	for i, header := range headers {
		style := lipgloss.NewStyle().Bold(true)
		if !f.NoColor {
			style = style.Foreground(f.Theme.Primary)
		}
		headerRow[i] = f.applyStyle(header, style)
	}
	rows = append(rows, headerRow)

	// Add data rows
	for _, item := range items {
		row, err := f.getTableRowFormatted(item, fields)
		if err != nil {
			continue // Skip invalid rows
		}
		rows = append(rows, row)
	}

	return f.formatTableRows(rows), nil
}

// getTableRowFormatted extracts a formatted row from a struct or map
func (f *PrettyFormatter) getTableRowFormatted(item interface{}, fields []api.PrettyField) ([]string, error) {
	val := reflect.ValueOf(item)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	row := make([]string, len(fields))

	for i, field := range fields {
		var fieldVal reflect.Value

		if val.Kind() == reflect.Struct {
			fieldVal = f.getFieldValue(val, field.Name)
		} else {
			return nil, fmt.Errorf("table items must be structs, got %v", val.Kind())
		}

		if !fieldVal.IsValid() {
			row[i] = ""
			continue
		}

		// Format the value using the field specification
		formatted := f.formatValue(fieldVal.Interface(), field)
		row[i] = formatted
	}

	return row, nil
}

// formatTableRows formats table rows with proper alignment
func (f *PrettyFormatter) formatTableRows(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}

	// Calculate column widths based on actual content
	colWidths := make([]int, len(rows[0]))
	minWidth := 8  // Reduced minimum column width for better fitting
	maxWidth := 50 // Maximum column width to prevent overly wide columns

	for _, row := range rows {
		for i, cell := range row {
			// Strip ANSI codes for width calculation
			cellWidth := len(f.stripAnsi(cell))
			if cellWidth > colWidths[i] {
				colWidths[i] = cellWidth
			}
		}
	}

	// Apply min/max constraints and add minimal padding
	for i := range colWidths {
		// Ensure minimum width
		if colWidths[i] < minWidth {
			colWidths[i] = minWidth
		}
		// Cap at maximum width
		if colWidths[i] > maxWidth {
			colWidths[i] = maxWidth
		}
		// Add minimal padding (2 spaces on each side)
		colWidths[i] += 2
	}

	// Create table style
	borderStyle := lipgloss.NewStyle()
	if !f.NoColor {
		borderStyle = borderStyle.Foreground(f.Theme.Muted)
	}

	var result strings.Builder

	// Top border
	result.WriteString(f.createTableBorder(colWidths, "┌", "┬", "┐", "─", borderStyle))
	result.WriteString("\n")

	// Header row
	if len(rows) > 0 {
		result.WriteString(f.formatTableRow(rows[0], colWidths, borderStyle))
		result.WriteString("\n")

		// Header separator
		if len(rows) > 1 {
			result.WriteString(f.createTableBorder(colWidths, "├", "┼", "┤", "─", borderStyle))
			result.WriteString("\n")
		}
	}

	// Data rows
	for i := 1; i < len(rows); i++ {
		result.WriteString(f.formatTableRow(rows[i], colWidths, borderStyle))
		result.WriteString("\n")
	}

	// Bottom border
	result.WriteString(f.createTableBorder(colWidths, "└", "┴", "┘", "─", borderStyle))

	return result.String()
}

// formatTableRow formats a single table row
func (f *PrettyFormatter) formatTableRow(row []string, colWidths []int, borderStyle lipgloss.Style) string {
	var result strings.Builder

	result.WriteString(f.applyStyle("│", borderStyle))
	for i, cell := range row {
		// Calculate remaining padding after cell content
		cellLen := len(f.stripAnsi(cell))
		totalPadding := colWidths[i] - cellLen

		// Distribute padding - 1 space before, rest after
		result.WriteString(" ")
		result.WriteString(cell)
		if totalPadding > 0 {
			result.WriteString(strings.Repeat(" ", totalPadding-1))
		}
		result.WriteString(f.applyStyle("│", borderStyle))
	}

	return result.String()
}

// createTableBorder creates a table border line
func (f *PrettyFormatter) createTableBorder(colWidths []int, left, mid, right, fill string, style lipgloss.Style) string {
	var result strings.Builder

	result.WriteString(f.applyStyle(left, style))
	for i, width := range colWidths {
		// Border width matches column width (includes padding)
		result.WriteString(f.applyStyle(strings.Repeat(fill, width), style))
		if i < len(colWidths)-1 {
			result.WriteString(f.applyStyle(mid, style))
		}
	}
	result.WriteString(f.applyStyle(right, style))

	return result.String()
}

// applyStyle applies a lipgloss style if colors are enabled
func (f *PrettyFormatter) applyStyle(text string, style lipgloss.Style) string {
	if f.NoColor {
		return text
	}
	return style.Render(text)
}

// applyFormatStyle applies styling based on format type
func (f *PrettyFormatter) applyFormatStyle(text string, format string) string {
	if f.NoColor {
		return text
	}
	
	var style lipgloss.Style
	switch format {
	case api.FormatCurrency:
		style = lipgloss.NewStyle().Foreground(f.Theme.Success)
	case api.FormatDate:
		style = lipgloss.NewStyle().Foreground(f.Theme.Info)
	case api.FormatFloat:
		style = lipgloss.NewStyle().Foreground(f.Theme.Secondary)
	default:
		return text
	}
	
	return f.applyStyle(text, style)
}

// stripAnsi removes ANSI escape codes for width calculation
func (f *PrettyFormatter) stripAnsi(s string) string {
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

// getColorStyle returns lipgloss style for a color name
func (f *PrettyFormatter) getColorStyle(color string) lipgloss.Style {
	style := lipgloss.NewStyle()
	if !f.NoColor {
		switch color {
		case "green":
			style = style.Foreground(f.Theme.Success)
		case "red":
			style = style.Foreground(f.Theme.Error)
		case "blue":
			style = style.Foreground(f.Theme.Info)
		case "yellow":
			style = style.Foreground(f.Theme.Warning)
		default:
			style = style.Foreground(lipgloss.Color(color))
		}
	}
	return style
}


// toLipglossStyle converts a tailwind.Style to lipgloss.Style
func (f *PrettyFormatter) toLipglossStyle(style tailwind.Style) lipgloss.Style {
	lipStyle := lipgloss.NewStyle()
	
	if style.Foreground != "" {
		lipStyle = lipStyle.Foreground(lipgloss.Color(style.Foreground))
	}
	if style.Background != "" {
		lipStyle = lipStyle.Background(lipgloss.Color(style.Background))
	}
	if style.Bold {
		lipStyle = lipStyle.Bold(true)
	}
	if style.Faint {
		lipStyle = lipStyle.Faint(true)
	}
	if style.Italic {
		lipStyle = lipStyle.Italic(true)
	}
	if style.Underline {
		lipStyle = lipStyle.Underline(true)
	}
	if style.Strikethrough {
		lipStyle = lipStyle.Strikethrough(true)
	}
	if style.MaxWidth > 0 {
		lipStyle = lipStyle.MaxWidth(style.MaxWidth)
	}
	
	return lipStyle
}

// applyTailwindStyleToText applies both style and text transform to text
func (f *PrettyFormatter) applyTailwindStyleToText(text string, styleStr string) string {
	if f.NoColor {
		// If no color, still apply text transforms
		parsedStyle := tailwind.ParseStyle(styleStr)
		if parsedStyle.TextTransform != "" {
			text = tailwind.TransformText(text, parsedStyle.TextTransform)
		}
		return text
	}
	
	// Apply style and get transformed text
	transformedText, style := tailwind.ApplyStyle(text, styleStr)
	lipglossStyle := f.toLipglossStyle(style)
	return f.applyStyle(transformedText, lipglossStyle)
}

// formatPrettyData formats PrettyData without using reflection
func (f *PrettyFormatter) formatPrettyData(data *api.PrettyData) (string, error) {
	var sections []string
	var summaryFields []api.PrettyField
	var tableFields []api.PrettyField

	// Separate table fields from summary fields
	for _, field := range data.Schema.Fields {
		if field.Format == api.FormatTable {
			tableFields = append(tableFields, field)
		} else {
			summaryFields = append(summaryFields, field)
		}
	}

	// Format summary fields
	if len(summaryFields) > 0 {
		summaryOutput := f.formatSummaryFieldsData(summaryFields, data.Values)
		if summaryOutput != "" {
			sections = append(sections, summaryOutput)
		}
	}

	// Format tables
	for _, field := range tableFields {
		tableData, exists := data.Tables[field.Name]
		if exists && len(tableData) > 0 {
			tableOutput, err := f.formatTableData(tableData, field)
			if err != nil {
				return "", err
			}
			sections = append(sections, tableOutput)
		}
	}

	return strings.Join(sections, "\n"), nil
}

// formatSummaryFieldsData formats summary fields from FieldValues
func (f *PrettyFormatter) formatSummaryFieldsData(fields []api.PrettyField, values map[string]api.FieldValue) string {
	if len(fields) == 0 {
		return ""
	}

	// Create pairs of fields for 2-column layout
	var rows []string
	for i := 0; i < len(fields); i += 2 {
		leftField := fields[i]
		leftVal, leftExists := values[leftField.Name]

		var leftFormatted string
		if leftExists {
			leftFormatted = f.formatFieldValueData(leftField.Name, leftVal, leftField)
		} else {
			// Field not in data
			leftFormatted = f.formatMissingField(leftField.Name)
		}

		if i+1 < len(fields) {
			// Two columns
			rightField := fields[i+1]
			rightVal, rightExists := values[rightField.Name]

			var rightFormatted string
			if rightExists {
				rightFormatted = f.formatFieldValueData(rightField.Name, rightVal, rightField)
			} else {
				rightFormatted = f.formatMissingField(rightField.Name)
			}

			// Create side-by-side layout with wider columns
			leftColumn := lipgloss.NewStyle().Width(50).Render(leftFormatted)
			rightColumn := lipgloss.NewStyle().Width(50).Render(rightFormatted)
			row := lipgloss.JoinHorizontal(lipgloss.Left, leftColumn, rightColumn)
			rows = append(rows, row)
		} else {
			// Single column for odd number of fields
			rows = append(rows, leftFormatted)
		}
	}

	return strings.Join(rows, "\n")
}

// formatFieldValueData formats a single field value
func (f *PrettyFormatter) formatFieldValueData(name string, val api.FieldValue, field api.PrettyField) string {
	prettyName := f.prettifyFieldName(name)
	formatted := val.Formatted()

	// Apply field style if specified (highest priority)
	if field.Style != "" {
		formatted = f.applyTailwindStyleToText(formatted, field.Style)
	} else if color := val.Color(); color != "" {
		// Apply color styling using FieldValue.Color()
		style := f.getColorStyle(color)
		formatted = f.applyStyle(formatted, style)
	} else {
		// Apply format-specific styling
		formatted = f.applyFormatStyle(formatted, val.Field.Format)
	}

	// Apply label_style if specified, otherwise use default
	var styledLabel string
	if field.LabelStyle != "" {
		styledLabel = f.applyTailwindStyleToText(prettyName, field.LabelStyle)
	} else {
		// Default label style
		labelStyle := lipgloss.NewStyle().Bold(true)
		if !f.NoColor {
			labelStyle = labelStyle.Foreground(f.Theme.Primary)
		}
		styledLabel = f.applyStyle(prettyName, labelStyle)
	}

	return fmt.Sprintf("%s: %s", styledLabel, formatted)
}

// formatMissingField formats a field that has no data
func (f *PrettyFormatter) formatMissingField(name string) string {
	labelStyle := lipgloss.NewStyle().Bold(true)
	if !f.NoColor {
		labelStyle = labelStyle.Foreground(f.Theme.Primary)
	}

	prettyName := f.prettifyFieldName(name)
	nullStyle := lipgloss.NewStyle()
	if !f.NoColor {
		nullStyle = nullStyle.Foreground(f.Theme.Muted)
	}

	return fmt.Sprintf("%s: %s",
		f.applyStyle(prettyName, labelStyle),
		f.applyStyle("null", nullStyle))
}

// formatTableData formats table data without reflection
func (f *PrettyFormatter) formatTableData(rows []api.PrettyDataRow, field api.PrettyField) (string, error) {
	if len(rows) == 0 {
		return f.applyStyle("(empty table)", lipgloss.NewStyle().Foreground(f.Theme.Muted)), nil
	}

	// Get headers from table field definition
	headers := make([]string, len(field.TableOptions.Fields))
	for i, tableField := range field.TableOptions.Fields {
		headers[i] = tableField.Name
	}

	// Create table rows
	var tableRows [][]string

	// Add header row
	headerRow := make([]string, len(headers))
	for i, header := range headers {
		// Use header_style if specified
		if field.TableOptions.HeaderStyle != "" {
			headerRow[i] = f.applyTailwindStyleToText(header, field.TableOptions.HeaderStyle)
		} else {
			// Default header style
			style := lipgloss.NewStyle().Bold(true)
			if !f.NoColor {
				style = style.Foreground(f.Theme.Primary)
			}
			headerRow[i] = f.applyStyle(header, style)
		}
	}
	tableRows = append(tableRows, headerRow)

	// Add data rows
	for _, row := range rows {
		dataRow := make([]string, len(field.TableOptions.Fields))
		for i, tableField := range field.TableOptions.Fields {
			fieldValue, exists := row[tableField.Name]
			if exists {
				formatted := fieldValue.Formatted()

				// Apply individual field style first (highest priority)
				if tableField.Style != "" {
					formatted = f.applyTailwindStyleToText(formatted, tableField.Style)
				} else if field.TableOptions.RowStyle != "" {
					// Apply row_style if no individual field style
					formatted = f.applyTailwindStyleToText(formatted, field.TableOptions.RowStyle)
				} else if color := fieldValue.Color(); color != "" {
					// Apply color styling using FieldValue.Color()
					style := f.getColorStyle(color)
					formatted = f.applyStyle(formatted, style)
				} else {
					// Apply format-specific styling
					formatted = f.applyFormatStyle(formatted, tableField.Format)
				}

				dataRow[i] = formatted
			} else {
				dataRow[i] = ""
			}
		}
		tableRows = append(tableRows, dataRow)
	}

	return f.formatTableRows(tableRows), nil
}
