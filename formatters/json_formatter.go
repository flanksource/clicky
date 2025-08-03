package formatters

import (
	"encoding/json"
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
