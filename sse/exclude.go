package sse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// excludeWildcard is the Exclude segment that matches every map key or array
// element.
const excludeWildcard = "*"

// parseExcludes splits each dotted Exclude path into segments, panicking on a
// path that could never match anything.
func parseExcludes(paths []string) [][]string {
	parsed := make([][]string, 0, len(paths))
	for _, path := range paths {
		segments := strings.Split(path, ".")
		for _, segment := range segments {
			if segment == "" {
				panic(fmt.Sprintf("sse: SnapshotOptions.Exclude path %q has an empty segment", path))
			}
		}
		parsed = append(parsed, segments)
	}
	return parsed
}

// comparisonForm is payload with every excluded path removed, re-encoded
// canonically (encoding/json sorts map keys). Without excludes the payload is
// already canonical: json.Marshal output is deterministic for a given value.
func comparisonForm(payload []byte, excludes [][]string) ([]byte, error) {
	if len(excludes) == 0 {
		return payload, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	// Numbers stay as written, so a large integer never compares equal to a
	// neighbour it rounds to as a float64.
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode snapshot for comparison: %w", err)
	}
	for _, path := range excludes {
		value = stripPath(value, path)
	}
	compare, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode snapshot for comparison: %w", err)
	}
	return compare, nil
}

// stripPath removes path from value, returning the (possibly replaced) value.
// A path that does not exist in this value is not an error: optional and
// per-item fields are routinely absent.
func stripPath(value any, path []string) any {
	head, rest := path[0], path[1:]
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			if head != excludeWildcard && key != head {
				continue
			}
			if len(rest) == 0 {
				delete(node, key)
			} else {
				node[key] = stripPath(child, rest)
			}
		}
		return node
	case []any:
		if head != excludeWildcard {
			return node
		}
		if len(rest) == 0 {
			return []any{}
		}
		for i, child := range node {
			node[i] = stripPath(child, rest)
		}
		return node
	}
	return value
}
