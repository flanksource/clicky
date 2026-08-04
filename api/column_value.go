package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	ColumnTypeKeyValue  = "key_value"
	ColumnTypeKeyValues = "key_values"
	ColumnTypeJSON      = "json"
)

// IsStructuredColumnType reports whether a column type carries JSON-like
// structure that needs a semantic table-cell representation.
func IsStructuredColumnType(columnType string) bool {
	switch columnType {
	case ColumnTypeKeyValue, ColumnTypeKeyValues, ColumnTypeJSON:
		return true
	default:
		return false
	}
}

// ColumnTextable adapts a raw column value to a structured Clicky value. It is
// intentionally presentation-only: callers that emit JSON/YAML should retain
// the original raw value instead of replacing it with this representation.
func ColumnTextable(column ColumnDef, value any) Textable {
	switch column.Type {
	case ColumnTypeKeyValue, ColumnTypeKeyValues:
		if pairs, ok := normalizeKeyValuePairs(value); ok {
			return DescriptionList{Items: pairs, Style: "compact"}
		}
	case ColumnTypeJSON:
		if normalized, ok := normalizeJSONColumnValue(value); ok {
			if data, err := json.Marshal(normalized); err == nil {
				return CodeBlock("json", string(data))
			}
		}
	}
	if formatted, ok := formatColumnScalar(column, value); ok {
		return formatted
	}
	return convertToTextable(value)
}

// ColumnString returns the deterministic presentation value used by text
// exports such as CSV and Markdown. Structured JSON/YAML and numeric spreadsheet
// exporters should continue to encode the original raw value.
func ColumnString(column ColumnDef, value any) string {
	switch column.Type {
	case ColumnTypeKeyValue, ColumnTypeKeyValues:
		if pairs, ok := normalizeKeyValuePairs(value); ok {
			parts := make([]string, 0, len(pairs))
			for _, pair := range pairs {
				parts = append(parts, pair.Key+"="+columnScalarString(pair.Value))
			}
			return strings.Join(parts, ", ")
		}
	case ColumnTypeJSON:
		if normalized, ok := normalizeJSONColumnValue(value); ok {
			if data, err := json.Marshal(normalized); err == nil {
				return string(data)
			}
		}
	}
	return ColumnTextable(column, value).String()
}

func normalizeKeyValuePairs(value any) ([]KeyValuePair, bool) {
	if list, ok := value.(DescriptionList); ok {
		return list.Items, true
	}
	if list, ok := value.(*DescriptionList); ok && list != nil {
		return list.Items, true
	}

	decoded, ok := decodeStructuredColumnValue(value, true)
	if !ok {
		return nil, false
	}
	return keyValuePairsFromDecoded(decoded)
}

func keyValuePairsFromDecoded(value any) ([]KeyValuePair, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if pair, ok := keyValuePairFromObject(typed); ok {
			return []KeyValuePair{pair}, true
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		pairs := make([]KeyValuePair, 0, len(keys))
		for _, key := range keys {
			pairs = append(pairs, KeyValuePair{Key: key, Value: normalizedPairValue(typed[key]), Style: "compact"})
		}
		return pairs, true
	case []any:
		pairs := make([]KeyValuePair, 0, len(typed))
		for _, item := range typed {
			object, ok := item.(map[string]any)
			if !ok {
				return nil, false
			}
			if pair, ok := keyValuePairFromObject(object); ok {
				pairs = append(pairs, pair)
				continue
			}
			objectPairs, ok := keyValuePairsFromDecoded(object)
			if !ok {
				return nil, false
			}
			pairs = append(pairs, objectPairs...)
		}
		return pairs, true
	default:
		return nil, false
	}
}

func keyValuePairFromObject(object map[string]any) (KeyValuePair, bool) {
	key, hasKey := lookupObjectString(object, "key", "name")
	value, hasValue := lookupObjectValue(object, "value")
	if !hasKey || !hasValue {
		return KeyValuePair{}, false
	}
	return KeyValuePair{Key: key, Value: normalizedPairValue(value), Style: "compact"}, true
}

func lookupObjectString(object map[string]any, names ...string) (string, bool) {
	value, ok := lookupObjectValue(object, names...)
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	return text, ok
}

func lookupObjectValue(object map[string]any, names ...string) (any, bool) {
	for _, name := range names {
		for key, value := range object {
			if strings.EqualFold(key, name) {
				return value, true
			}
		}
	}
	return nil, false
}

func normalizedPairValue(value any) any {
	if value == nil {
		return "null"
	}
	if text, ok := value.(string); ok {
		if text == "" {
			return `""`
		}
		return text
	}
	switch value.(type) {
	case bool, float64, json.Number:
		return value
	}
	if data, err := json.Marshal(value); err == nil {
		return string(data)
	}
	return fmt.Sprintf("%v", value)
}

func columnScalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return "null"
	default:
		if data, err := json.Marshal(typed); err == nil {
			return string(data)
		}
		return fmt.Sprintf("%v", typed)
	}
}

func normalizeJSONColumnValue(value any) (any, bool) {
	if value == nil {
		return nil, true
	}
	return decodeStructuredColumnValue(value, false)
}

func decodeStructuredColumnValue(value any, requireContainer bool) (any, bool) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) && json.Valid([]byte(trimmed)) {
			var decoded any
			if json.Unmarshal([]byte(trimmed), &decoded) == nil {
				return decoded, true
			}
		}
		if requireContainer {
			return nil, false
		}
		return typed, true
	case []byte:
		if json.Valid(typed) {
			var decoded any
			if json.Unmarshal(typed, &decoded) == nil {
				if !requireContainer || isJSONContainer(decoded) {
					return decoded, true
				}
			}
		}
		if requireContainer {
			return nil, false
		}
		return string(typed), true
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return nil, false
		}
		var decoded any
		if err := json.Unmarshal(data, &decoded); err != nil {
			return nil, false
		}
		if requireContainer && !isJSONContainer(decoded) {
			return nil, false
		}
		return decoded, true
	}
}

func isJSONContainer(value any) bool {
	switch value.(type) {
	case map[string]any, []any:
		return true
	default:
		return false
	}
}
