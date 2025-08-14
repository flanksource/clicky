package formatters

import (
	"encoding/csv"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// CSVFormatter handles CSV formatting
type CSVFormatter struct {
	Separator rune
}

// NewCSVFormatter creates a new CSV formatter
func NewCSVFormatter() *CSVFormatter {
	return &CSVFormatter{
		Separator: ',',
	}
}

// Format formats data as CSV
func (f *CSVFormatter) Format(data interface{}) (string, error) {
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Handle slice/array of structs or maps
	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		return f.formatSlice(val)
	}

	// Handle single struct
	if val.Kind() == reflect.Struct {
		return f.formatStruct(val)
	}

	// Handle single map
	if val.Kind() == reflect.Map {
		return f.formatMap(val)
	}

	// Fallback for other types
	return fmt.Sprintf("%v", data), nil
}

// formatSlice formats a slice of structs as CSV
func (f *CSVFormatter) formatSlice(val reflect.Value) (string, error) {
	if val.Len() == 0 {
		return "", nil
	}

	var output strings.Builder
	writer := csv.NewWriter(&output)
	writer.Comma = f.Separator

	// Get the first item to determine headers
	firstItem := val.Index(0)
	if firstItem.Kind() == reflect.Ptr {
		firstItem = firstItem.Elem()
	}

	// Handle interface{} values by getting the underlying value
	if firstItem.Kind() == reflect.Interface && !firstItem.IsNil() {
		firstItem = firstItem.Elem()
	}

	if firstItem.Kind() != reflect.Struct && firstItem.Kind() != reflect.Map {
		return "", fmt.Errorf("CSV formatting requires slice of structs or maps, got %v", firstItem.Kind())
	}

	// Write headers
	headers := f.getStructHeaders(firstItem)
	if err := writer.Write(headers); err != nil {
		return "", err
	}

	// Write data rows
	for i := 0; i < val.Len(); i++ {
		item := val.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}

		// Handle interface{} values by getting the underlying value
		if item.Kind() == reflect.Interface && !item.IsNil() {
			item = item.Elem()
		}

		row := f.getStructRow(item)
		if err := writer.Write(row); err != nil {
			return "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	return output.String(), nil
}

// formatStruct formats a single struct as CSV
func (f *CSVFormatter) formatStruct(val reflect.Value) (string, error) {
	return f.formatSingleRow(f.getStructHeaders(val), f.getStructRow(val))
}

// formatMap formats a single map as CSV
func (f *CSVFormatter) formatMap(val reflect.Value) (string, error) {
	return f.formatSingleRow(f.getMapHeaders(val), f.getMapRow(val))
}

// formatSingleRow formats headers and a single data row as CSV
func (f *CSVFormatter) formatSingleRow(headers []string, row []string) (string, error) {
	var output strings.Builder
	writer := csv.NewWriter(&output)
	writer.Comma = f.Separator

	// Write headers
	if err := writer.Write(headers); err != nil {
		return "", err
	}

	// Write data row
	if err := writer.Write(row); err != nil {
		return "", err
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	return output.String(), nil
}

// getStructHeaders extracts field names as CSV headers from structs or maps
func (f *CSVFormatter) getStructHeaders(val reflect.Value) []string {
	if val.Kind() == reflect.Map {
		return f.getMapHeaders(val)
	}

	return GetStructHeaders(val)
}

// getMapHeaders extracts keys as CSV headers from maps (sorted for consistency)
func (f *CSVFormatter) getMapHeaders(val reflect.Value) []string {
	if val.Kind() != reflect.Map {
		return nil
	}

	var headers []string
	for _, key := range val.MapKeys() {
		if key.Kind() == reflect.String {
			headers = append(headers, key.String())
		}
	}

	// Sort headers for consistent output
	sort.Strings(headers)
	return headers
}

// getStructRow extracts field values as CSV row from structs or maps
func (f *CSVFormatter) getStructRow(val reflect.Value) []string {
	if val.Kind() == reflect.Map {
		return f.getMapRow(val)
	}

	return GetStructRow(val)
}

// getMapRow extracts values as CSV row from maps (using the same key order as headers)
func (f *CSVFormatter) getMapRow(val reflect.Value) []string {
	if val.Kind() != reflect.Map {
		return nil
	}

	headers := f.getMapHeaders(val)
	row := make([]string, len(headers))

	for i, header := range headers {
		mapVal := val.MapIndex(reflect.ValueOf(header))
		if mapVal.IsValid() {
			if mapVal.Kind() == reflect.Interface && !mapVal.IsNil() {
				mapVal = mapVal.Elem()
			}
			row[i] = fmt.Sprintf("%v", mapVal.Interface())
		} else {
			row[i] = ""
		}
	}

	return row
}
