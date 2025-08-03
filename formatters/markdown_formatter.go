package formatters

import (
	"fmt"
	"reflect"
	"strings"
)

// MarkdownFormatter handles Markdown formatting
type MarkdownFormatter struct{}

// NewMarkdownFormatter creates a new Markdown formatter
func NewMarkdownFormatter() *MarkdownFormatter {
	return &MarkdownFormatter{}
}

// Format formats data as Markdown
func (f *MarkdownFormatter) Format(data interface{}) (string, error) {
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Handle slice/array of structs as table
	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		return f.formatSliceAsTable(val)
	}

	// Handle single struct
	if val.Kind() == reflect.Struct {
		return f.formatStructAsDefinitionList(val), nil
	}

	// Fallback for other types
	return fmt.Sprintf("```\n%v\n```", data), nil
}

// formatSliceAsTable formats a slice of structs as Markdown table
func (f *MarkdownFormatter) formatSliceAsTable(val reflect.Value) (string, error) {
	if val.Len() == 0 {
		return "*No data*", nil
	}

	// Get the first item to determine headers
	firstItem := val.Index(0)
	if firstItem.Kind() == reflect.Ptr {
		firstItem = firstItem.Elem()
	}

	if firstItem.Kind() != reflect.Struct {
		return "", fmt.Errorf("Markdown table formatting requires slice of structs")
	}

	var result strings.Builder

	// Write headers
	headers := f.getStructHeaders(firstItem)
	result.WriteString("| ")
	result.WriteString(strings.Join(headers, " | "))
	result.WriteString(" |\n")

	// Write separator
	result.WriteString("|")
	for range headers {
		result.WriteString(" --- |")
	}
	result.WriteString("\n")

	// Write data rows
	for i := 0; i < val.Len(); i++ {
		item := val.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}

		row := f.getStructRow(item)
		result.WriteString("| ")

		// Escape pipe characters in cell content
		escapedRow := make([]string, len(row))
		for j, cell := range row {
			escapedRow[j] = strings.ReplaceAll(cell, "|", "\\|")
		}

		result.WriteString(strings.Join(escapedRow, " | "))
		result.WriteString(" |\n")
	}

	return result.String(), nil
}

// formatStructAsDefinitionList formats a struct as Markdown definition list
func (f *MarkdownFormatter) formatStructAsDefinitionList(val reflect.Value) string {
	var result strings.Builder

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if !fieldVal.CanInterface() {
			continue
		}

		// Skip hidden fields
		prettyTag := field.Tag.Get("pretty")
		if prettyTag == "hide" {
			continue
		}

		// Get field name
		fieldName := field.Name
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			if parts := strings.Split(jsonTag, ","); parts[0] != "" {
				fieldName = parts[0]
			}
		}

		value := fmt.Sprintf("%v", fieldVal.Interface())
		result.WriteString(fmt.Sprintf("**%s**: %s\n\n", fieldName, value))
	}

	return result.String()
}

// getStructHeaders extracts field names
func (f *MarkdownFormatter) getStructHeaders(val reflect.Value) []string {
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
		if prettyTag == "hide" {
			continue
		}

		// Get field name
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

// getStructRow extracts field values
func (f *MarkdownFormatter) getStructRow(val reflect.Value) []string {
	typ := val.Type()
	var row []string

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if !fieldVal.CanInterface() {
			continue
		}

		// Skip hidden fields
		prettyTag := field.Tag.Get("pretty")
		if prettyTag == "hide" {
			continue
		}

		value := fmt.Sprintf("%v", fieldVal.Interface())
		row = append(row, value)
	}

	return row
}
