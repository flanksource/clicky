package formatters

import (
	"encoding/json"

	"github.com/flanksource/clicky/api"
)

// JSONFormatter handles JSON formatting
type JSONFormatter struct {
	Indent string
}

// NewJSONFormatter creates a new JSON formatter
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{
		Indent: "  ",
	}
}

// Format formats data as JSON
func (f *JSONFormatter) Format(data interface{}) (string, error) {
	// Convert to PrettyData and use FormatPrettyData
	prettyData, err := ToPrettyData(data)
	if err != nil || prettyData == nil || prettyData.Original == nil {
		// Fallback to direct YAML serialization
		return "", err
	}
	return f.FormatPrettyData(prettyData)
}

// FormatPrettyData formats PrettyData as JSON using the original data if available
func (f *JSONFormatter) FormatPrettyData(data *api.PrettyData) (string, error) {
	if data == nil {
		return "null", nil
	}

	return f.formatValue(data.Original)
}

// formatValue is a helper to format any value as JSON
func (f *JSONFormatter) formatValue(data interface{}) (string, error) {
	if b, err := json.MarshalIndent(data, "", f.Indent); err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}

// FormatCompact formats data as compact JSON (no indentation)
func (f *JSONFormatter) FormatCompact(data interface{}) (string, error) {
	if b, err := json.Marshal(data); err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}
