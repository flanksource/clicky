package formatters

import (
	"encoding/json"

	"github.com/alpkeskin/gotoon"

	"github.com/flanksource/clicky/api"
)

// TOONFormatter renders data as Token-Oriented Object Notation: a uniform
// collection names its fields once in a header rather than once per row, which
// is where TOON's saving over JSON comes from when the reader is an LLM.
type TOONFormatter struct{}

// NewTOONFormatter creates a new TOON formatter
func NewTOONFormatter() *TOONFormatter {
	return &TOONFormatter{}
}

// Format formats data as TOON
func (f *TOONFormatter) Format(data interface{}) (string, error) {
	return f.FormatValue(data)
}

// FormatPrettyData formats PrettyData as TOON using the original data if available
func (f *TOONFormatter) FormatPrettyData(data *api.PrettyData) (string, error) {
	if data == nil {
		return "null\n", nil
	}

	return f.FormatValue(data.Original)
}

// FormatValue is a helper to format any value as TOON.
//
// The value is decoded through encoding/json on the way in. gotoon can walk a
// struct itself, but it takes the whole json tag as the field name — so
// `json:"id,omitempty"` becomes the key `id,omitempty` — honours neither
// omitempty nor `json:"-"`, and never calls MarshalJSON, which would spell a
// uuid.UUID as its sixteen raw bytes. Decoding first means TOON names, elides
// and renders exactly the fields --format json does.
func (f *TOONFormatter) FormatValue(data interface{}) (string, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", err
	}

	encoded, err := gotoon.Encode(decoded)
	if err != nil {
		return "", err
	}

	// gotoon joins lines without a trailing newline; every other line-oriented
	// formatter here ends with one.
	return encoded + "\n", nil
}
