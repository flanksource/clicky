package prompt

import (
	"encoding/json"
	"strconv"
)

// The generated select/multi-select forms encode the option *index* as the enum
// value ("0","1",…) and carry the human labels in x-enum-labels, so a chosen
// answer maps back to an index without depending on label uniqueness. clicky-ui's
// JsonSchemaForm renders x-enum-labels + x-enum-display.

const choiceKey = "choice"
const textKey = "value"

func indexEnum(options []string) ([]any, map[string]string) {
	enum := make([]any, len(options))
	labels := make(map[string]string, len(options))
	for i, opt := range options {
		key := strconv.Itoa(i)
		enum[i] = key
		labels[key] = opt
	}
	return enum, labels
}

// SelectSchema builds an object schema with a single required radio "choice" whose
// value is the selected option's index.
func SelectSchema(title string, options []string) json.RawMessage {
	enum, labels := indexEnum(options)
	doc := map[string]any{
		"type": "object",
		"properties": map[string]any{
			choiceKey: map[string]any{
				"type":           "string",
				"title":          title,
				"enum":           enum,
				"x-enum-labels":  labels,
				"x-enum-display": "radio",
			},
		},
		"required": []string{choiceKey},
	}
	b, _ := json.Marshal(doc)
	return b
}

// MultiSelectSchema builds an object schema whose "choice" is an array of option
// indices. uniqueItems rejects a repeated selection; maxItems (when > 0) caps the
// number of selections so the sink/dashboard path enforces the same limit as the
// terminal multi-select.
func MultiSelectSchema(title string, options []string, maxItems int) json.RawMessage {
	enum, labels := indexEnum(options)
	choice := map[string]any{
		"type":  "array",
		"title": title,
		"items": map[string]any{
			"type":          "string",
			"enum":          enum,
			"x-enum-labels": labels,
		},
		"uniqueItems":    true,
		"x-enum-display": "checkbox",
	}
	if maxItems > 0 {
		choice["maxItems"] = maxItems
	}
	doc := map[string]any{
		"type":       "object",
		"properties": map[string]any{choiceKey: choice},
		"required":   []string{choiceKey},
	}
	b, _ := json.Marshal(doc)
	return b
}

// TextSchema builds an object schema with a single required free-text "value".
func TextSchema(title string, secret bool) json.RawMessage {
	value := map[string]any{"type": "string", "title": title}
	if secret {
		value["format"] = "password"
	}
	doc := map[string]any{
		"type":       "object",
		"properties": map[string]any{textKey: value},
		"required":   []string{textKey},
	}
	b, _ := json.Marshal(doc)
	return b
}

// ConfirmSchema builds an object schema with a single boolean "value".
func ConfirmSchema(title string) json.RawMessage {
	doc := map[string]any{
		"type": "object",
		"properties": map[string]any{
			textKey: map[string]any{"type": "boolean", "title": title},
		},
		"required": []string{textKey},
	}
	b, _ := json.Marshal(doc)
	return b
}

// SelectedIndex extracts the chosen option index from a SelectSchema answer, or
// -1 if absent/unparseable.
func SelectedIndex(ans Answer) int {
	s, _ := ans.Values[choiceKey].(string)
	i, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return i
}

// SelectedIndexes extracts the chosen option indices from a MultiSelectSchema
// answer.
func SelectedIndexes(ans Answer) []int {
	raw, ok := ans.Values[choiceKey].([]any)
	if !ok {
		return nil
	}
	out := make([]int, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			if i, err := strconv.Atoi(s); err == nil {
				out = append(out, i)
			}
		}
	}
	return out
}

// TextValue extracts the entered string from a TextSchema answer.
func TextValue(ans Answer) string {
	s, _ := ans.Values[textKey].(string)
	return s
}

// ConfirmValue extracts the boolean from a ConfirmSchema answer.
func ConfirmValue(ans Answer) bool {
	b, _ := ans.Values[textKey].(bool)
	return b
}
