package api

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/samber/lo"
)

// Pretty enables objects to provide rich text formatting with styling and structure.
// Types implementing this interface can control their visual representation across
// different output formats (terminal, HTML, PDF, etc.).
type Pretty interface {
	Pretty() Text
}

// PrettyRow enables structs to provide custom table row representation with fine-grained control
// over columns and cell formatting based on output format options.
// The opts parameter should be of type formatters.FormatOptions.
type PrettyRow interface {
	PrettyRow(opts interface{}) map[string]Text
}

// RenderFunc provides custom rendering logic for field values.
// It receives the raw value, field configuration, and current theme,
// allowing complete control over how a field is displayed.
type RenderFunc func(value interface{}, field PrettyField, theme Theme) string

// PrettyField configures how a data field should be formatted and displayed.
// It supports schema-driven formatting with type inference, custom styling,
// nested field definitions, and conditional coloring based on field values.
type PrettyField struct {
	Name          string            `json:"name" yaml:"name"`
	Aliases       []string          `json:"aliases,omitempty" yaml:"aliases,omitempty"`
	Type          string            `json:"type,omitempty" yaml:"type,omitempty"`
	Format        string            `json:"format,omitempty" yaml:"format,omitempty"`
	Label         string            `json:"label,omitempty" yaml:"label,omitempty"`
	Default       string            `json:"default,omitempty" yaml:"default,omitempty"`
	Style         string            `json:"style,omitempty" yaml:"style,omitempty"`
	LabelStyle    string            `json:"label_style,omitempty" yaml:"label_style,omitempty"`
	Color         string            `json:"color,omitempty" yaml:"color,omitempty"`
	DateFormat    string            `json:"date_format,omitempty" yaml:"date_format,omitempty"`
	FormatOptions map[string]string `json:"format_options,omitempty" yaml:"format_options,omitempty"`
	ColorOptions  map[string]string `json:"color_options,omitempty" yaml:"color_options,omitempty"`
	// For nested struct fields
	Fields []PrettyField `json:"fields,omitempty" yaml:"fields,omitempty"`
	// For table formatting
	TableOptions TableOptions `json:"table_options,omitempty" yaml:"table_options,omitempty"`
	// For tree formatting
	TreeOptions *TreeOptions `json:"tree_options,omitempty" yaml:"tree_options,omitempty"`
	// For custom rendering
	RenderFunc   RenderFunc `json:"-" yaml:"-"`
	CompactItems bool       `json:"compact_items,omitempty" yaml:"compact_items,omitempty"`
}

// TableOptions configures tabular data presentation including column definitions,
// sorting behavior, and styling options for headers and rows.
type TableOptions struct {
	Title         string                   `json:"title,omitempty" yaml:"title,omitempty"`
	Columns       []PrettyField            `json:"fields" yaml:"fields"`
	Rows          []map[string]interface{} `json:"rows,omitempty" yaml:"rows,omitempty"`
	SortField     string                   `json:"sort_field,omitempty" yaml:"sort_field,omitempty"`
	SortDirection string                   `json:"sort_direction,omitempty" yaml:"sort_direction,omitempty"`
	HeaderStyle   string                   `json:"header_style,omitempty" yaml:"header_style,omitempty"`
	RowStyle      string                   `json:"row_style,omitempty" yaml:"row_style,omitempty"`
}

// PrettyObject defines the schema for formatting structured data,
// containing field definitions that control how each property is displayed.
type PrettyObject struct {
	Fields []PrettyField `json:"fields" yaml:"fields"`
}

type PrettyStructData struct {
}

// FieldValue wraps a raw value with type-safe accessors and formatting metadata.
// It provides strongly-typed access to primitive values (string, int, float, bool, time)
// while maintaining the original value and supporting rich text output.
type FieldValue struct {
	Field        PrettyField
	Value        interface{} // Can be *PrettyData for nested structures
	StringValue  *string
	IntValue     *int64
	FloatValue   *float64
	BooleanValue *bool
	TimeValue    *time.Time
	ArrayValue   []interface{}
	MapValue     map[string]interface{}
	Text         Textable
	Tree         TreeNode
}

var epoch = time.Now().Add(-50 * 365 * 24 * time.Hour)
var centuryEnd = time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)

func isValidTimeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	if t.Before(epoch) || t.After(centuryEnd) {
		return nil
	}
	return &t
}

func ParseEpoch(e any) *time.Time {
	v := FieldValue{Value: e}
	if f := v.Float(); f != nil {
		n := int64(*f)

		if n > epoch.UnixNano() {
			seconds := n / int64(time.Second)
			nanosRemainder := n % int64(time.Second)
			return isValidTimeOrNil(time.Unix(seconds, nanosRemainder))
		} else if n > epoch.UnixMilli() {
			return isValidTimeOrNil(time.UnixMilli(n))
		}
	}
	return nil
}

func (v FieldValue) DateTimeFormat() string {
	var format = v.Field.DateFormat
	if format == "" {
		if f, ok := v.Field.FormatOptions["format"]; ok {
			format = f
		}
	}
	if format == "" || format == "epoch" {
		return "2006-01-02 15:04:05"
	}
	return format
}

func (v FieldValue) Time() *time.Time {
	// Return already parsed time if available
	if v.TimeValue != nil {
		return v.TimeValue
	}

	// Try to parse from Value
	switch val := v.Value.(type) {
	case time.Time:
		return &val
	case string:
		e := ParseEpoch(val)
		if e != nil {
			return e
		}

		if t, err := time.Parse(v.DateTimeFormat(), val); err == nil {
			return &t
		} else if t, err := time.Parse(time.RFC3339, val); err == nil {
			return &t
		} else if t, err := time.Parse("2006-01-02 15:04:05", val); err == nil {
			return &t
		} else if t, err := time.Parse("2006-01-02", val); err == nil {
			return &t
		}
	}

	if f := ParseEpoch(v.Value); f != nil {
		return f
	}

	return nil
}

func (v FieldValue) Float() *float64 {

	if v.FloatValue != nil {
		return v.FloatValue
	}

	if v.IntValue != nil {
		return lo.ToPtr(float64(*v.IntValue))
	}

	switch val := v.Value.(type) {
	case float64:
		return lo.ToPtr(val)
	case int64:
		return lo.ToPtr(float64(val))
	case int32:
		return lo.ToPtr(float64(int64(val)))
	case int:
		return lo.ToPtr(float64(int64(val)))
	case string:
		if i, err := strconv.ParseFloat(val, 64); err == nil {
			return lo.ToPtr(i)
		}
	}

	return nil
}

func (v FieldValue) Int() *int64 {
	if v.IntValue != nil {
		return v.IntValue
	}

	i := v.Float()
	if i == nil {
		return nil
	}
	return lo.ToPtr(int64(*i))
}

// formatCurrency formats a value as currency
func (v FieldValue) formatCurrency() string {
	// Get currency symbol from format options or default to $
	symbol := "$"
	if sym, ok := v.Field.FormatOptions["symbol"]; ok {
		symbol = sym
	}

	if v.FloatValue != nil {
		return fmt.Sprintf("%s%.2f", symbol, *v.FloatValue)
	}
	if v.IntValue != nil {
		return fmt.Sprintf("%s%d.00", symbol, *v.IntValue)
	}

	// Try to parse from Value
	if val, ok := v.Value.(float64); ok {
		return fmt.Sprintf("%s%.2f", symbol, val)
	}
	if val, ok := v.Value.(int); ok {
		return fmt.Sprintf("%s%d.00", symbol, val)
	}

	return fmt.Sprintf("%v", v.Value)
}

// formatDate formats a value as a date
func (v FieldValue) formatDate() string {

	if t := v.Time(); t != nil {
		return t.Format(v.DateTimeFormat())
	}
	return ""
}

// formatFloat formats a float value
func (v FieldValue) formatFloat() string {
	digits := 2
	if d, ok := v.Field.FormatOptions["digits"]; ok {
		if parsed, err := strconv.Atoi(d); err == nil {
			digits = parsed
		}
	}

	format := fmt.Sprintf("%%.%df", digits)

	if v.Float() != nil {
		return fmt.Sprintf(format, *v.Float())
	}
	return ""
}

// formatDuration formats a duration value
func (v FieldValue) formatDuration() string {
	if val, ok := v.Value.(time.Duration); ok {
		return val.String()
	}

	// Try to parse as int64 (nanoseconds)
	if val, ok := v.Value.(int64); ok {
		return time.Duration(val).String()
	}

	return fmt.Sprintf("%v", v.Value)
}

// formatArray formats an array value
func (v FieldValue) formatArray() string {
	if v.ArrayValue != nil {
		strs := make([]string, len(v.ArrayValue))
		for i, item := range v.ArrayValue {
			strs[i] = fmt.Sprintf("%v", item)
		}
		return "[" + strings.Join(strs, ", ") + "]"
	}

	// Try reflection for other slice types
	val := reflect.ValueOf(v.Value)
	if val.Kind() == reflect.Slice {
		strs := make([]string, val.Len())
		for i := 0; i < val.Len(); i++ {
			strs[i] = fmt.Sprintf("%v", val.Index(i).Interface())
		}
		return "[" + strings.Join(strs, ", ") + "]"
	}

	return fmt.Sprintf("%v", v.Value)
}

// Formatted returns the formatted string representation of the field value
func (v FieldValue) Formatted() string {
	if v.Text != nil {
		return v.Text.String()
	}
	if v.StringValue != nil {
		return *v.StringValue
	}
	return fmt.Sprintf("%v", v.Value)
}

// Plain returns the plain string representation of the field value
func (v FieldValue) Plain() string {
	return v.Formatted()
}

// Markdown returns the markdown representation of the field value
func (v FieldValue) Markdown() string {
	if v.Text != nil {
		return v.Text.Markdown()
	}
	return v.Formatted()
}

// HTML returns the HTML representation of the field value
func (v FieldValue) HTML() string {
	if v.Text != nil {
		return v.Text.HTML()
	}
	return v.Formatted()
}

// Color determines the display color by matching the field value against
// ColorOptions patterns, supporting exact matches and numeric comparisons.
func (v FieldValue) Color() string {
	if v.Field.Color != "" {
		return v.Field.Color
	}

	// Check color options for matching values
	valueStr := v.Formatted()
	for color, pattern := range v.Field.ColorOptions {
		if v.matchesColorPattern(valueStr, pattern) {
			return color
		}
	}

	return ""
}

// matchesColorPattern checks if a value matches a color pattern
func (v FieldValue) matchesColorPattern(value, pattern string) bool {
	// Handle exact match
	if value == pattern {
		return true
	}

	// Handle numeric comparisons
	if strings.HasPrefix(pattern, ">=") || strings.HasPrefix(pattern, ">") ||
		strings.HasPrefix(pattern, "<=") || strings.HasPrefix(pattern, "<") {
		return v.matchesNumericPattern(value, pattern)
	}

	// Handle pattern matching (simple contains for now)
	return strings.Contains(strings.ToLower(value), strings.ToLower(pattern))
}

// matchesNumericPattern handles numeric comparison patterns
func (v FieldValue) matchesNumericPattern(value, pattern string) bool {
	// Extract operator and threshold
	var op string
	var thresholdStr string

	if strings.HasPrefix(pattern, ">=") {
		op = ">="
		thresholdStr = strings.TrimSpace(pattern[2:])
	} else if strings.HasPrefix(pattern, "<=") {
		op = "<="
		thresholdStr = strings.TrimSpace(pattern[2:])
	} else if strings.HasPrefix(pattern, ">") {
		op = ">"
		thresholdStr = strings.TrimSpace(pattern[1:])
	} else if strings.HasPrefix(pattern, "<") {
		op = "<"
		thresholdStr = strings.TrimSpace(pattern[1:])
	} else {
		return false
	}

	// Parse threshold
	threshold, err := strconv.ParseFloat(thresholdStr, 64)
	if err != nil {
		return false
	}

	// Parse value
	var numValue float64
	if v.FloatValue != nil {
		numValue = *v.FloatValue
	} else if v.IntValue != nil {
		numValue = float64(*v.IntValue)
	} else {
		// Try parsing from string
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return false
		}
		numValue = parsed
	}

	// Compare
	switch op {
	case ">=":
		return numValue >= threshold
	case ">":
		return numValue > threshold
	case "<=":
		return numValue <= threshold
	case "<":
		return numValue < threshold
	}

	return false
}

// Primitive extracts the underlying typed value, returning the most specific
// type available (string, int64, float64, bool, time.Time) or the raw value.
func (v FieldValue) Primitive() interface{} {
	if v.StringValue != nil {
		return *v.StringValue
	}
	if v.IntValue != nil {
		return *v.IntValue
	}
	if v.FloatValue != nil {
		return *v.FloatValue
	}
	if v.BooleanValue != nil {
		return *v.BooleanValue
	}
	if v.TimeValue != nil {
		return *v.TimeValue
	}
	return v.Value
}

// Parse converts a raw value into a FieldValue with type inference and validation.
// It performs type conversion based on the field's configured type, handles nested
// structures, and creates appropriate Text objects for rich formatting.
func (f PrettyField) Parse(value interface{}) (FieldValue, error) {
	v := FieldValue{
		Field: f,
		Value: value,
	}

	if value == nil {
		return v, nil
	}

	// Check if value implements TreeNode interface - if so, preserve the pointer
	if _, ok := value.(TreeNode); ok {
		// Store TreeNode as-is without dereferencing
		return v, nil
	}

	// Dereference pointer values before processing
	val := reflect.ValueOf(value)
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return v, nil
		}
		val = val.Elem()
		value = val.Interface()
		v.Value = value
	}

	// Get the actual type for parsing
	actualType := f.Type
	if actualType == "" {
		actualType = InferValueType(value)
	}

	// Check for type mismatch between schema and actual data
	inferredType := InferValueType(value)
	if actualType == FieldTypeStruct && inferredType == FieldTypeMap {
		actualType = "map"
	}

	// Handle nested struct/map fields
	if actualType == FieldTypeStruct || actualType == FieldTypeMap {
		// For nested structures, we'll handle them separately
		// The parser will create nested FieldValues
		return v, nil
	}

	// Type conversion based on field type
	switch actualType {
	case FieldTypeString:
		if str, ok := value.(string); ok {
			v.StringValue = &str
		} else {
			str := fmt.Sprintf("%v", value)
			v.StringValue = &str
		}

	case FieldTypeInt:
		switch val := value.(type) {
		case int:
			i := int64(val)
			v.IntValue = &i
		case int64:
			v.IntValue = &val
		case float64:
			i := int64(val)
			v.IntValue = &i
		case string:
			if i, err := strconv.ParseInt(val, 10, 64); err == nil {
				v.IntValue = &i
			}
		}

	case FieldTypeFloat:
		switch val := value.(type) {
		case float64:
			v.FloatValue = &val
		case float32:
			f := float64(val)
			v.FloatValue = &f
		case int:
			f := float64(val)
			v.FloatValue = &f
		case string:
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				v.FloatValue = &f
			}
		}

	case FieldTypeBoolean:
		switch val := value.(type) {
		case bool:
			v.BooleanValue = &val
		case string:
			if b, err := strconv.ParseBool(val); err == nil {
				v.BooleanValue = &b
			}
		}

	case FieldTypeDate:
		switch val := value.(type) {
		case time.Time:
			v.TimeValue = &val
		case string:
			// Try parsing as Unix timestamp (integer)
			if timestamp, err := strconv.ParseInt(val, 10, 64); err == nil {
				t := time.Unix(timestamp, 0)
				v.TimeValue = &t
			} else if timestamp, err := strconv.ParseFloat(val, 64); err == nil {
				// Try parsing as Unix timestamp (float)
				t := time.Unix(int64(timestamp), 0)
				v.TimeValue = &t
			} else {
				// Try various date string formats
				dateFormats := []string{
					time.RFC3339,
					DateTimeFormat,
					"2006-01-02",
					time.RFC3339Nano,
					"2006-01-02T15:04:05",
				}

				var parsed time.Time
				var parseErr error
				for _, format := range dateFormats {
					if parsed, parseErr = time.Parse(format, val); parseErr == nil {
						v.TimeValue = &parsed
						break
					}
				}
				// If parsing fails, TimeValue remains nil
			}
		case int:
			// Unix timestamp as int
			t := time.Unix(int64(val), 0)
			v.TimeValue = &t
		case int64:
			// Unix timestamp
			t := time.Unix(val, 0)
			v.TimeValue = &t
		case float64:
			// Unix timestamp with possible milliseconds
			sec := int64(val)
			nsec := int64((val - float64(sec)) * 1e9)
			t := time.Unix(sec, nsec)
			v.TimeValue = &t
		}

	case FieldTypeArray:
		val := reflect.ValueOf(value)
		if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
			v.ArrayValue = make([]interface{}, val.Len())
			for i := 0; i < val.Len(); i++ {
				v.ArrayValue[i] = val.Index(i).Interface()
			}
		}

	case FieldTypeMap:
		// For maps, we'll store the raw value and format it specially
		if mapVal, ok := value.(map[string]interface{}); ok {
			v.MapValue = mapVal
		}
	}

	// Ensure ArrayValue and MapValue are always populated for arrays and maps,
	// regardless of the actualType in the switch above
	valType := reflect.ValueOf(value)
	if valType.Kind() == reflect.Slice || valType.Kind() == reflect.Array {
		if v.ArrayValue == nil {
			v.ArrayValue = make([]interface{}, valType.Len())
			for i := 0; i < valType.Len(); i++ {
				v.ArrayValue[i] = valType.Index(i).Interface()
			}
		}
	} else if valType.Kind() == reflect.Map {
		if v.MapValue == nil {
			// Convert any map type to map[string]interface{}
			v.MapValue = make(map[string]interface{})
			iter := valType.MapRange()
			for iter.Next() {
				k := iter.Key()
				v.MapValue[fmt.Sprintf("%v", k.Interface())] = iter.Value().Interface()
			}
		}
	}

	// Create Text object with appropriate formatting and styling
	v.Text = v.createText()

	return v, nil
}

// createText creates a Text object with appropriate formatting and styling
func (v FieldValue) createText() *Text {
	// Handle null values
	if v.Value == nil {
		return &Text{
			Content: "null",
			Style:   "text-gray-400", // Muted color for null values
		}
	}

	// Handle nested PrettyData structures
	if prettyData, ok := v.Value.(*PrettyData); ok {
		// Recursively format the nested PrettyData
		// This will be handled by the formatter calling code
		// For now, just indicate it's a nested structure
		return &Text{
			Content: fmt.Sprintf("(nested: %d fields)", len(prettyData.Values)),
		}
	}

	var content string
	var style string

	// Check for MapValue and ArrayValue first (populated regardless of field type)
	if v.MapValue != nil {
		pairs := make([]string, 0, len(v.MapValue))
		for k, val := range v.MapValue {
			pairs = append(pairs, fmt.Sprintf("%s: %v", k, val))
		}
		content = "{" + strings.Join(pairs, ", ") + "}"
	} else if v.ArrayValue != nil {
		content = v.formatArray()
	} else {
		// Format based on field format
		switch v.Field.Format {
		case "currency":
			content = v.formatCurrency()
			style = "text-green-600 font-medium" // Green for currency
		case FieldTypeDate:
			content = v.formatDate()
			style = "text-blue-600" // Blue for dates
		case FieldTypeFloat:
			content = v.formatFloat()
			style = "text-purple-600" // Purple for numbers
		case FieldTypeDuration:
			content = v.formatDuration()
			style = "text-orange-600" // Orange for durations
		case FieldTypeArray:
			content = v.formatArray()
		case FieldTypeBytes:
			d := v.Int()
			if d != nil {
				content = HumanizeBytes(*d).String()
				style = "text-purple-600" // Cyan for byte sizes
			}
		default:
			// Default formatting based on type
			switch v.Field.Type {
			case FieldTypeString:
				if v.StringValue != nil {
					content = *v.StringValue
				} else {
					content = fmt.Sprintf("%v", v.Value)
				}
			case FieldTypeInt:
				if v.IntValue != nil {
					content = fmt.Sprintf("%d", *v.IntValue)
				} else {
					content = fmt.Sprintf("%v", v.Value)
				}
			case FieldTypeFloat:
				if v.FloatValue != nil {
					content = fmt.Sprintf("%.6g", *v.FloatValue)
				} else {
					content = fmt.Sprintf("%v", v.Value)
				}
			case FieldTypeBoolean:
				if v.BooleanValue != nil {
					content = fmt.Sprintf("%v", *v.BooleanValue)
				} else {
					content = fmt.Sprintf("%v", v.Value)
				}
			case FieldTypeDate:
				content = v.formatDate()
				style = "text-blue-600"
			case FieldTypeArray:
				content = v.formatArray()
			case FieldTypeMap:
				if v.MapValue != nil {
					pairs := make([]string, 0, len(v.MapValue))
					for k, val := range v.MapValue {
						pairs = append(pairs, fmt.Sprintf("%s: %v", k, val))
					}
					content = "{" + strings.Join(pairs, ", ") + "}"
				} else {
					content = fmt.Sprintf("%v", v.Value)
				}
			default:
				content = fmt.Sprintf("%v", v.Value)
			}
		}
	}

	// Apply custom style from field if specified
	if v.Field.Style != "" {
		style = v.Field.Style
	} else if v.Field.Color != "" {
		style = v.Field.Color
	}

	return &Text{
		Content: content,
		Style:   style,
	}
}

// InferValueType determines the appropriate field type for a given value
// using reflection and type assertions, returning standard type constants.
func InferValueType(value interface{}) string {
	if value == nil {
		return "nil"
	}

	// Use reflection to check for maps and slices
	val := reflect.ValueOf(value)

	switch val.Kind() {
	case reflect.Map:
		return FieldTypeMap
	case reflect.Struct:
		// Check for time.Time
		if _, ok := value.(time.Time); ok {
			return FieldTypeDate
		}
		// Check for time.Duration
		if _, ok := value.(time.Duration); ok {
			return FieldTypeDuration
		}
		return FieldTypeStruct
	case reflect.Slice, reflect.Array:
		return FieldTypeArray
	case reflect.String:
		return FieldTypeString
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return FieldTypeInt
	case reflect.Float32, reflect.Float64:
		return FieldTypeFloat
	case reflect.Bool:
		return FieldTypeBoolean
	default:
		// Also check concrete types
		switch value.(type) {
		case string:
			return FieldTypeString
		case int, int64:
			return FieldTypeInt
		case float64, float32:
			return FieldTypeFloat
		case bool:
			return FieldTypeBoolean
		case time.Time:
			return FieldTypeDate
		case time.Duration:
			return FieldTypeDuration
		case map[string]interface{}:
			return FieldTypeMap
		default:
			return "unknown"
		}
	}
}

// prettifyFieldName converts field names to readable format (for map keys)
func (f PrettyField) prettifyFieldName(name string) string {
	// Convert snake_case and camelCase to Title Case
	var result strings.Builder
	words := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-'
	})

	if len(words) == 0 {
		// Handle camelCase
		words = f.splitCamelCase(name)
	}

	for i, word := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(cases.Title(language.English).String(strings.ToLower(word)))
	}

	return result.String()
}

// splitCamelCase splits camelCase strings into words
func (f PrettyField) splitCamelCase(s string) []string {
	var words []string
	var current strings.Builder

	for i, r := range s {
		if i > 0 && (r >= 'A' && r <= 'Z') {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		}
		current.WriteRune(r)
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

// IsTableField returns true if this field represents a table
func (v FieldValue) IsTableField() bool {
	return v.Field.Format == "table"
}

func (v FieldValue) IsTreeField() bool {
	return v.Field.Format == "tree"
}

// GetFieldType returns the type of the field
func (v FieldValue) GetFieldType() string {
	return v.Field.Type
}

// RenderFuncRegistry stores named custom render functions
var RenderFuncRegistry = map[string]RenderFunc{}

// RegisterRenderFunc adds a named custom render function to the global registry.
// These functions can be referenced in field configurations for specialized formatting.
func RegisterRenderFunc(name string, fn RenderFunc) {
	RenderFuncRegistry[name] = fn
}

// Pretty converts PrettyData to a TextList representation.
// This is the primary method for formatters - call .JoinNewlines().ANSI()/HTML()/Markdown() on the result.
func (pd *PrettyData) Pretty() TextList {
	if pd == nil {
		return TextList{}
	}

	list := TextList{}

	// Process regular fields
	for _, field := range pd.Schema.Fields {
		if field.Format == FormatHide || field.Format == FormatTable {
			continue
		}

		fieldValue, ok := pd.Values[field.Name]
		if !ok {
			continue
		}

		label := field.Label
		if label == "" {
			label = PrettifyFieldName(field.Name)
		}

		// Handle nested PrettyData structures
		if nestedData, ok := fieldValue.Value.(*PrettyData); ok {
			// Add label with colon on its own line
			labelText := Text{}.
				Append(label, "text-purple-500").
				Append(":", "text-muted")
			list = append(list, labelText)

			// Add indented nested content
			nestedList := nestedData.Pretty()
			for _, nestedItem := range nestedList.Indent() {
				list = append(list, nestedItem)
			}
		} else {
			// Build field text: "Label: Value"
			fieldText := Text{}.
				Append(label, "text-purple-500").
				Append(": ", "text-muted")
			if fieldValue.Text != nil {
				fieldText = fieldText.Add(fieldValue.Text)
			} else {
				fieldText = fieldText.Append(fieldValue.Formatted(), "")
			}

			list = append(list, fieldText)
		}
	}

	// Note: Table fields are skipped here and handled separately by formatters
	// Phase 3 will implement proper Table object generation in parsers

	return list
}

// FormatManager defines the interface for converting data to various output formats.
// Implementations handle the complete pipeline from raw data to formatted output
// across multiple formats (JSON, YAML, CSV, Markdown, HTML, etc.).
type FormatManager interface {
	ToPrettyData(data interface{}) (*PrettyData, error)
	Pretty(data interface{}) (string, error)
	JSON(data interface{}) (string, error)
	YAML(data interface{}) (string, error)
	CSV(data interface{}) (string, error)
	Markdown(data interface{}) (string, error)
	HTML(data interface{}) (string, error)
}
