package formatters

import (
	"encoding/json"
	"reflect"
	"strings"

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
	return f.FormatValue(data)
}

// FormatPrettyData formats PrettyData as JSON using the original data if available
func (f *JSONFormatter) FormatPrettyData(data *api.PrettyData) (string, error) {
	if data == nil {
		return "null", nil
	}

	return f.FormatValue(data.Original)
}

// FormatValue is a helper to format any value as JSON
func (f *JSONFormatter) FormatValue(data interface{}) (string, error) {
	if b, err := json.MarshalIndent(data, "", f.Indent); err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}

// FormatLines formats data as JSON Lines (ndjson): one compact JSON value per
// line, a slice emitting one line per element.
//
// The encoder is deliberately encoding/json's, configured exactly as
// writeJSONStream's is — HTML escaping on, a trailing newline per value — so a
// CLI `--format ndjson` and the streaming export of the same rows are the same
// bytes rather than merely the same shape.
func (f *JSONFormatter) FormatLines(data interface{}) (string, error) {
	// Unwrap single-element varargs slices, as the other formatters do.
	if slice, ok := data.([]interface{}); ok && len(slice) == 1 {
		data = slice[0]
	}

	var out strings.Builder
	enc := json.NewEncoder(&out)
	value := reflect.ValueOf(data)
	if data != nil && (value.Kind() == reflect.Slice || value.Kind() == reflect.Array) &&
		value.Type().Elem().Kind() != reflect.Uint8 {
		for i := 0; i < value.Len(); i++ {
			if err := enc.Encode(value.Index(i).Interface()); err != nil {
				return "", err
			}
		}
		return out.String(), nil
	}

	if err := enc.Encode(data); err != nil {
		return "", err
	}
	return out.String(), nil
}

// FormatCompact formats data as compact JSON (no indentation)
func (f *JSONFormatter) FormatCompact(data interface{}) (string, error) {
	if b, err := json.Marshal(data); err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}
