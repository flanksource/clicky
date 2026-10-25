package schemaform

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// Values held by the form are JSON-shaped: string, float64, bool, nil, []any
// and map[string]any. Numbers are always float64 so const/enum comparisons and
// listener conditions see the same representation a decoded JSON answer has.

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func asObject(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

// normalize converts Go-native values (ints, typed slices and maps) into the
// JSON shape, by round-tripping anything that is not already JSON-shaped.
func normalize(v any) any {
	switch t := v.(type) {
	case nil, string, bool, float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case int32:
		return float64(t)
	case float32:
		return float64(t)
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return t.String()
		}
		return f
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = normalize(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, item := range t {
			out[k] = normalize(item)
		}
		return out
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return v
	}
	return out
}

func jsonEqual(a, b any) bool {
	return reflect.DeepEqual(normalize(a), normalize(b))
}

func cloneValue(v any) any {
	switch t := v.(type) {
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = cloneValue(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, item := range t {
			out[k] = cloneValue(item)
		}
		return out
	}
	return normalize(v)
}

// isEmpty mirrors clicky-ui's submitValue drop rule: a value that carries no
// data is not submitted. false and 0 are data.
func isEmpty(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	}
	return false
}

// formatScalar renders a scalar for a text input or an enum option value.
func formatScalar(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(raw)
}

// parseScalar parses text typed into a field whose schema type is typ.
func parseScalar(text string, typ TypeList) (any, error) {
	switch {
	case typ.Has("integer"):
		n, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil || n != float64(int64(n)) {
			return nil, fmt.Errorf("%q is not an integer", text)
		}
		return n, nil
	case typ.Has("number"):
		n, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number", text)
		}
		return n, nil
	case typ.Has("boolean"):
		b, err := strconv.ParseBool(strings.TrimSpace(text))
		if err != nil {
			return nil, fmt.Errorf("%q is not a boolean", text)
		}
		return b, nil
	}
	return text, nil
}

// splitList splits a tag-list entry: one value per line, or comma separated on
// a single line (the shape accessible mode and CLI users type).
func splitList(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		for _, part := range strings.Split(line, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func getPath(root map[string]any, path []string) (any, bool) {
	var current any = root
	for _, seg := range path {
		object, ok := asObject(current)
		if !ok {
			return nil, false
		}
		if current, ok = object[seg]; !ok {
			return nil, false
		}
	}
	return current, true
}

// setPath writes v at path, creating intermediate objects; a nil v deletes.
func setPath(root map[string]any, path []string, v any) {
	object := root
	for _, seg := range path[:len(path)-1] {
		child, ok := asObject(object[seg])
		if !ok {
			child = map[string]any{}
			object[seg] = child
		}
		object = child
	}
	last := path[len(path)-1]
	if v == nil {
		delete(object, last)
		return
	}
	object[last] = v
}

func joinPath(path []string) string { return strings.Join(path, ".") }

func appendPath(path []string, key string) []string {
	return append(append([]string(nil), path...), key)
}
