package api

import "encoding/json"

// Keyed wraps a Textable with a data key. Rendering passes through to the
// wrapped value unchanged in every format; only JSON serialization differs —
// the value marshals under Key as a single-field object. Use it when a block in
// a document needs a stable identifier for structured output (e.g. a table keyed
// by its heading) without altering how it renders.
type Keyed struct {
	Key   string
	Value Textable
}

func (k Keyed) String() string   { return textableString(k.Value) }
func (k Keyed) ANSI() string     { return textableANSI(k.Value) }
func (k Keyed) HTML() string     { return textableHTML(k.Value) }
func (k Keyed) Markdown() string { return textableMarkdown(k.Value) }

// MarshalJSON emits {Key: Value}. The wrapped value serializes via its own
// MarshalJSON when it implements json.Marshaler, otherwise via its rendered
// string.
func (k Keyed) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{k.Key: jsonValue(k.Value)})
}

func textableString(t Textable) string {
	if t == nil {
		return ""
	}
	return t.String()
}

func textableANSI(t Textable) string {
	if t == nil {
		return ""
	}
	return t.ANSI()
}

func textableHTML(t Textable) string {
	if t == nil {
		return ""
	}
	return t.HTML()
}

func textableMarkdown(t Textable) string {
	if t == nil {
		return ""
	}
	return t.Markdown()
}

// jsonValue returns the wrapped value as-is when it can marshal itself (so a
// TextTable serializes to its row array, an Amount to its formatted string),
// falling back to the rendered string otherwise.
func jsonValue(t Textable) any {
	if t == nil {
		return nil
	}
	if _, ok := t.(json.Marshaler); ok {
		return t
	}
	return t.String()
}
