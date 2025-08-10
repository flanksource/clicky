package clicky

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
)

// PrettyParser handles parsing of structs with pretty tags
type PrettyParser struct {
	Theme   api.Theme
	NoColor bool
}

// NewPrettyParser creates a new parser with default theme
func NewPrettyParser() *PrettyParser {
	return &PrettyParser{
		Theme: api.DefaultTheme(),
	}
}

// Parse parses a struct and returns formatted output
func (p *PrettyParser) Parse(data interface{}) (string, error) {
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

// parseStruct processes a struct and its tags
func (p *PrettyParser) parseStruct(val reflect.Value) (string, error) {
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
		if prettyTag == "hide" {
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

		// Handle table formatting
		if prettyField.Format == "table" {
			if fieldVal.Kind() == reflect.Slice {
				tableOutput, err := p.formatTable(fieldVal, prettyField)
				if err != nil {
					return "", err
				}
				fields = append(fields, tableOutput)
				continue
			}
		}

		formatted := p.formatField(fieldName, fieldVal, prettyField)
		fields = append(fields, formatted)
	}

	return strings.Join(fields, "\n"), nil
}

// parsePrettyTag parses the pretty tag into a PrettyField
func (p *PrettyParser) parsePrettyTag(tag string) api.PrettyField {
	return api.ParsePrettyTag(tag)
}

// formatField formats a single field
func (p *PrettyParser) formatField(name string, val reflect.Value, field api.PrettyField) string {
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
func (p *PrettyParser) formatValue(val reflect.Value, field api.PrettyField) string {
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
	case "tree":
		return p.formatAsTree(val, field)
	default:
		return p.formatDefault(val)
	}
}

// formatCurrency formats a value as currency
func (p *PrettyParser) formatCurrency(val reflect.Value) string {
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
func (p *PrettyParser) formatDate(val reflect.Value, format string) string {
	style := lipgloss.NewStyle()
	if !p.NoColor {
		style = style.Foreground(p.Theme.Info)
	}

	var t time.Time
	var err error

	switch val.Kind() {
	case reflect.String:
		str := val.String()
		if format == "epoch" {
			if epoch, parseErr := strconv.ParseInt(str, 10, 64); parseErr == nil {
				t = time.Unix(epoch, 0)
			} else {
				return p.applyStyle(str, style)
			}
		} else {
			if t, err = time.Parse(time.RFC3339, str); err != nil {
				if t, err = time.Parse("2006-01-02", str); err != nil {
					return p.applyStyle(str, style)
				}
			}
		}
	case reflect.Int64:
		t = time.Unix(val.Int(), 0)
	default:
		return p.formatDefault(val)
	}

	return p.applyStyle(t.Format("2006-01-02 15:04:05"), style)
}

// formatFloat formats a float with specified digits
func (p *PrettyParser) formatFloat(val reflect.Value, digits string) string {
	style := lipgloss.NewStyle()
	if !p.NoColor {
		style = style.Foreground(p.Theme.Secondary)
	}

	precision := 2
	if digits != "" {
		if p, err := strconv.Atoi(digits); err == nil {
			precision = p
		}
	}

	switch val.Kind() {
	case reflect.Float32, reflect.Float64:
		format := fmt.Sprintf("%%.%df", precision)
		return p.applyStyle(fmt.Sprintf(format, val.Float()), style)
	default:
		return p.formatDefault(val)
	}
}

// formatWithColor formats a value with conditional colors
func (p *PrettyParser) formatWithColor(val reflect.Value, colorOptions map[string]string) string {
	valueStr := fmt.Sprintf("%v", val.Interface())

	// Find matching color condition
	for color, condition := range colorOptions {
		if p.matchesCondition(val, condition) {
			style := lipgloss.NewStyle()
			if !p.NoColor {
				switch color {
				case api.ColorGreen:
					style = style.Foreground(p.Theme.Success)
				case "red":
					style = style.Foreground(p.Theme.Error)
				case "blue":
					style = style.Foreground(p.Theme.Info)
				case "yellow":
					style = style.Foreground(p.Theme.Warning)
				default:
					style = style.Foreground(lipgloss.Color(color))
				}
			}
			return p.applyStyle(valueStr, style)
		}
	}

	return valueStr
}

// matchesCondition checks if a value matches a color condition
func (p *PrettyParser) matchesCondition(val reflect.Value, condition string) bool {
	valueStr := fmt.Sprintf("%v", val.Interface())

	// Exact string match
	if valueStr == condition {
		return true
	}

	// Numeric comparisons
	if strings.HasPrefix(condition, ">") || strings.HasPrefix(condition, "<") {
		return p.matchesNumericCondition(val, condition)
	}

	return false
}

// matchesNumericCondition checks numeric conditions like ">0", "<100"
func (p *PrettyParser) matchesNumericCondition(val reflect.Value, condition string) bool {
	var op string
	var threshold float64
	var err error

	if strings.HasPrefix(condition, ">=") {
		op = ">="
		threshold, err = strconv.ParseFloat(condition[2:], 64)
	} else if strings.HasPrefix(condition, "<=") {
		op = "<="
		threshold, err = strconv.ParseFloat(condition[2:], 64)
	} else if strings.HasPrefix(condition, ">") {
		op = ">"
		threshold, err = strconv.ParseFloat(condition[1:], 64)
	} else if strings.HasPrefix(condition, "<") {
		op = "<"
		threshold, err = strconv.ParseFloat(condition[1:], 64)
	} else {
		return false
	}

	if err != nil {
		return false
	}

	var value float64
	switch val.Kind() {
	case reflect.Float32, reflect.Float64:
		value = val.Float()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value = float64(val.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value = float64(val.Uint())
	default:
		return false
	}

	switch op {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	}

	return false
}

// formatDefault formats a value using default formatting
func (p *PrettyParser) formatDefault(val reflect.Value) string {
	return fmt.Sprintf("%v", val.Interface())
}

// formatTable formats a slice as a table
func (p *PrettyParser) formatTable(val reflect.Value, field api.PrettyField) (string, error) {
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
func (p *PrettyParser) sortSlice(items []interface{}, fieldName string, direction string) {
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
func (p *PrettyParser) getFieldValue(item interface{}, fieldName string) interface{} {
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
func (p *PrettyParser) compareValues(a, b interface{}) bool {
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
func (p *PrettyParser) renderTable(items []interface{}) (string, error) {
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
func (p *PrettyParser) getTableHeaders(item interface{}) ([]string, error) {
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
		if prettyTag == "hide" {
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
func (p *PrettyParser) getTableRow(item interface{}, headers []string) ([]string, error) {
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
		if prettyTag == "hide" {
			continue
		}

		if headerIndex >= len(headers) {
			break
		}

		// Parse pretty tag for formatting
		prettyField := p.parsePrettyTag(prettyTag)
		prettyField.Name = field.Name

		// Format the value
		formatted := p.formatValue(fieldVal, prettyField)
		row[headerIndex] = formatted
		headerIndex++
	}

	return row, nil
}

// formatTableRows formats table rows with proper alignment
func (p *PrettyParser) formatTableRows(rows [][]string) string {
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
func (p *PrettyParser) formatTableRow(row []string, colWidths []int, borderStyle lipgloss.Style) string {
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
func (p *PrettyParser) createTableBorder(colWidths []int, left, mid, right, fill string, style lipgloss.Style) string {
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

// applyStyle applies a lipgloss style if colors are enabled
func (p *PrettyParser) applyStyle(text string, style lipgloss.Style) string {
	if p.NoColor {
		return text
	}
	return style.Render(text)
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
func (p *PrettyParser) formatAsTree(val reflect.Value, field api.PrettyField) string {
	// Create tree formatter
	formatter := formatters.NewTreeFormatter(p.Theme, p.NoColor, field.TreeOptions)
	
	// Convert value to tree node
	var node api.TreeNode
	
	// Check if value already implements TreeNode
	if val.CanInterface() {
		if treeNode, ok := val.Interface().(api.TreeNode); ok {
			node = treeNode
		} else {
			// Try to convert to tree node
			node = formatters.ConvertToTreeNode(val.Interface())
		}
	}
	
	if node == nil {
		return p.formatDefault(val)
	}
	
	// Format the tree
	return formatter.FormatTreeFromRoot(node)
}
