package formatters

import (
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/flanksource/clicky/api"
)

func init() {
	csvFormatter := NewCSVFormatter()
	RegisterFormatter("csv", csvFormatter.Format)
}

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
func (f *CSVFormatter) Format(data interface{}, _ FormatOptions) (string, error) {
	// Unwrap single-element slices from varargs
	if slice, ok := data.([]interface{}); ok && len(slice) == 1 {
		data = slice[0]
	}

	// Handle TextTable directly
	switch v := data.(type) {
	case *api.TextTable:
		return f.FormatTable(v)
	case api.TextTable:
		return f.FormatTable(&v)
	}

	// Check if data implements Pretty interface first
	if pretty, ok := data.(api.Pretty); ok {
		text := pretty.Pretty()
		return text.String(), nil // Use plain text for CSV
	}

	// Convert to PrettyData (handles both structs and slices)
	prettyData, err := ToPrettyData(data)
	if err != nil {
		return "", fmt.Errorf("failed to convert to PrettyData: %w", err)
	}

	if prettyData.IsEmpty() {
		return "", nil
	}

	return f.FormatPrettyData(prettyData)
}

// FormatTable formats a TextTable as CSV
func (f *CSVFormatter) FormatTable(table *api.TextTable) (string, error) {
	if table == nil {
		return "", nil
	}

	var output strings.Builder
	writer := csv.NewWriter(&output)
	writer.Comma = f.Separator

	if err := writer.Write(table.Headers.AsString()); err != nil {
		return "", fmt.Errorf("failed to write CSV headers: %w", err)
	}

	for _, row := range table.Rows {
		if err := writer.Write(table.AsString(row)); err != nil {
			return "", fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	return output.String(), nil
}

// FormatPrettyData formats PrettyData as CSV, flattening all fields
func (f *CSVFormatter) FormatPrettyData(data *api.PrettyData) (string, error) {
	if data.IsEmpty() {
		return "", nil
	}

	table := data.FirstTable()
	if table == nil {
		return "", fmt.Errorf("no tables defined")
	}

	return f.FormatTable(table)
}
