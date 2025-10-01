package clicky

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ParseArgumentsAsMap parses HTTPie-style command line arguments into a map
// Supports:
//
//	key=value     - String values
//	key:=value    - JSON values (numbers, booleans, arrays, objects)
//	key==value    - Query parameters (returned in separate map)
//	key@file      - Read value from file
//	key:=@file    - Read JSON from file
//	Header:value  - HTTP headers (ignored in this function)
//	key[sub]=val  - Nested JSON structures
func ParseArgumentsAsMap(args []string) (map[string]any, error) {
	data, _, _, err := ParseArgumentsComplete(args)
	return data, err
}

// ParseArgumentsWithHeaders parses arguments and separates headers
func ParseArgumentsWithHeaders(args []string) (map[string]any, map[string]string, error) {
	data, headers, _, err := ParseArgumentsComplete(args)
	return data, headers, err
}

// ParseArgumentsComplete parses all argument types and returns separated results
func ParseArgumentsComplete(args []string) (map[string]any, map[string]string, map[string]string, error) {
	data := make(map[string]any)
	headers := make(map[string]string)
	query := make(map[string]string)

	for _, rawArg := range args {
		// Handle escaped arguments BEFORE parsing
		arg := unescapeArgument(rawArg)

		// Check for query parameter (but not if == is escaped)
		if isQueryParameter(rawArg) {
			parts := strings.SplitN(arg, "==", 2)
			if len(parts) == 2 {
				query[parts[0]] = parts[1]
			}
			continue
		}

		// Check for headers (but not if colon is escaped)
		if isHeaderParameter(rawArg) {
			colonIdx := strings.Index(arg, ":")
			key := arg[:colonIdx]
			value := arg[colonIdx+1:]

			if strings.HasPrefix(value, "@") {
				// Header from file: Header:@file
				filepath := value[1:]
				content, err := readFileAsString(filepath)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("reading header file '%s': %w", filepath, err)
				}
				headers[key] = strings.TrimSpace(content)
			} else {
				// Direct header value
				headers[key] = value
			}
			continue
		}

		// Parse other argument types (data fields) - pass the raw version for position detection
		key, value, argType, err := parseArgumentEnhanced(rawArg)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("parsing '%s': %w", arg, err)
		}

		if key != "" && argType == "data" {
			// Handle nested bracket notation
			if strings.Contains(key, "[") {
				err = setNestedValue(data, key, value)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("setting nested value '%s': %w", key, err)
				}
			} else {
				data[key] = value
			}
		}
	}

	return data, headers, query, nil
}

// ParseArgumentsWithQuery parses arguments and separates query parameters (backward compatibility)
func ParseArgumentsWithQuery(args []string) (data map[string]any, query map[string]string, err error) {
	data, _, query, err = ParseArgumentsComplete(args)
	return data, query, err
}

// parseArgumentEnhanced parses a single argument and returns key, value, type, and error
func parseArgumentEnhanced(rawArg string) (string, any, string, error) {
	// Work with the raw argument for position detection, but unescape for final values

	// Array from stdin: []key=- or key[]=-
	if strings.Contains(rawArg, "=-") {
		var key string
		if strings.HasPrefix(rawArg, "[]") && strings.Contains(rawArg, "=-") {
			// []key=- format
			idx := strings.Index(rawArg, "=-")
			key = unescapeArgument(rawArg[2:idx])
		} else if strings.Contains(rawArg, "[]=-") {
			// key[]=- format
			idx := strings.Index(rawArg, "[]=-")
			key = unescapeArgument(rawArg[:idx])
		}

		if key != "" {
			lines, err := readLinesFromFile("-")
			if err != nil {
				return "", nil, "", fmt.Errorf("reading stdin: %w", err)
			}
			return key, lines, "data", nil
		}
	}

	// Array from file: []key=@file or key[]=@file
	if strings.Contains(rawArg, "=@") && !strings.Contains(rawArg, ":=@") {
		var key, filepath string
		if strings.HasPrefix(rawArg, "[]") {
			// []key=@file format
			idx := strings.Index(rawArg, "=@")
			key = unescapeArgument(rawArg[2:idx])
			filepath = rawArg[idx+2:] // Skip the =@
		} else if strings.Contains(rawArg, "[]=@") {
			// key[]=@file format
			idx := strings.Index(rawArg, "[]=@")
			key = unescapeArgument(rawArg[:idx])
			filepath = rawArg[idx+4:] // Skip the []=@
		}

		if key != "" && filepath != "" {
			lines, err := readLinesFromFile(filepath)
			if err != nil {
				return "", nil, "", fmt.Errorf("reading file %s: %w", filepath, err)
			}
			return key, lines, "data", nil
		}
	}

	// JSON from file: key:=@file.json
	if idx := strings.Index(rawArg, ":=@"); idx > 0 {
		key := unescapeArgument(rawArg[:idx])
		filepath := rawArg[idx+3:]

		content, err := os.ReadFile(filepath)
		if err != nil {
			return "", nil, "", fmt.Errorf("reading file %s: %w", filepath, err)
		}

		var value any
		if err := json.Unmarshal(content, &value); err != nil {
			return "", nil, "", fmt.Errorf("parsing JSON from %s: %w", filepath, err)
		}

		return key, value, "data", nil
	}

	// String or binary from file: key@file (check for escaped @)
	if idx := strings.Index(rawArg, "@"); idx > 0 && !strings.Contains(rawArg[:idx], "=") && !isEscaped(rawArg, "@") {
		key := unescapeArgument(rawArg[:idx])
		filepath := rawArg[idx+1:]

		value, err := readFileAsStringOrBase64(filepath)
		if err != nil {
			return "", nil, "", fmt.Errorf("reading file %s: %w", filepath, err)
		}

		return key, value, "data", nil
	}

	// JSON value: key:=value
	if idx := strings.Index(rawArg, ":="); idx > 0 {
		key := unescapeArgument(rawArg[:idx])
		jsonStr := rawArg[idx+2:]

		var value any
		if err := json.Unmarshal([]byte(jsonStr), &value); err != nil {
			return "", nil, "", fmt.Errorf("parsing JSON value '%s': %w", jsonStr, err)
		}

		return key, value, "data", nil
	}

	// Query parameters are handled at a higher level, no need to check here

	// String value: key=value (but handle escaped equals in key)
	if idx := findUnescapedEquals(rawArg); idx > 0 {
		// Use the raw argument to find position, but unescape the parts
		rawKey := rawArg[:idx]
		rawValue := rawArg[idx+1:]
		key := unescapeArgument(rawKey)
		value := unescapeArgument(rawValue)
		return key, value, "data", nil
	}

	return "", nil, "", fmt.Errorf("invalid argument format, expected key=value, key:=json, key@file, Header:value, or key:=@file")
}

// Helper functions

// readFileAsString reads a file and returns its content as a string
func readFileAsString(filepath string) (string, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// readFileAsStringOrBase64 reads a file and returns it as string or base64 if binary
func readFileAsStringOrBase64(filepath string) (string, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}

	// Check if content is valid UTF-8 text
	if utf8.Valid(content) {
		// For text files, return as string
		return string(content), nil
	}

	// For binary files, return as base64
	return base64.StdEncoding.EncodeToString(content), nil
}

// readLinesFromFile reads lines from a file or stdin and returns them as a slice
// Skips empty lines and lines starting with #
func readLinesFromFile(filepath string) ([]string, error) {
	var content []byte
	var err error

	if filepath == "-" {
		// Read from stdin
		content, err = os.ReadFile("/dev/stdin")
	} else {
		// Read from file
		content, err = os.ReadFile(filepath)
	}

	if err != nil {
		return nil, err
	}

	var lines []string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}

	return lines, nil
}

// isQueryParameter checks if an argument is a query parameter (key==value) and not escaped
func isQueryParameter(arg string) bool {
	return strings.Contains(arg, "==") && !isEscaped(arg, "==")
}

// isHeaderParameter checks if an argument is a header (Key:value) and not escaped
func isHeaderParameter(arg string) bool {
	colonIdx := strings.Index(arg, ":")
	if colonIdx <= 0 {
		return false
	}

	// Must not have = before the colon
	if strings.Contains(arg[:colonIdx], "=") {
		return false
	}

	// Must not be :=
	if colonIdx+1 < len(arg) && arg[colonIdx+1] == '=' {
		return false
	}

	// Must not be escaped
	return !isEscaped(arg, ":")
}

// findUnescapedEquals finds the first unescaped = in the argument for key=value parsing
func findUnescapedEquals(arg string) int {
	for i := 0; i < len(arg); i++ {
		if arg[i] == '=' {
			// Count consecutive backslashes before this =
			backslashCount := 0
			for j := i - 1; j >= 0 && arg[j] == '\\'; j-- {
				backslashCount++
			}

			// If even number of backslashes (including 0), the = is not escaped
			if backslashCount%2 == 0 {
				return i
			}
		}
	}
	return -1
}

// isEscaped checks if a pattern is escaped in the argument
func isEscaped(arg, pattern string) bool {
	idx := strings.Index(arg, pattern)
	if idx <= 0 {
		return false
	}

	// Count consecutive backslashes before the pattern
	backslashCount := 0
	for i := idx - 1; i >= 0 && arg[i] == '\\'; i-- {
		backslashCount++
	}

	// If odd number of backslashes, the pattern is escaped
	return backslashCount%2 == 1
}

// unescapeArgument handles escape sequences in arguments
func unescapeArgument(arg string) string {
	// Handle basic escape sequences
	result := strings.ReplaceAll(arg, "\\=", "=")
	result = strings.ReplaceAll(result, "\\:", ":")
	result = strings.ReplaceAll(result, "\\@", "@")
	result = strings.ReplaceAll(result, "\\\\", "\\")
	return result
}

// setNestedValue sets a value in a nested map using bracket notation like "user[profile][name]"
func setNestedValue(data map[string]any, key string, value any) error {
	// Parse bracket notation: user[profile][name] or items[]
	bracketRegex := regexp.MustCompile(`^([^[]+)(\[[^\]]*\])+$`)
	if !bracketRegex.MatchString(key) {
		return fmt.Errorf("invalid bracket notation: %s", key)
	}

	// Extract base key (everything before first bracket)
	firstBracket := strings.Index(key, "[")
	baseKey := key[:firstBracket]

	// Extract all bracket parts from the entire key
	bracketRegex2 := regexp.MustCompile(`\[([^\]]*)\]`)
	bracketMatches := bracketRegex2.FindAllStringSubmatch(key, -1)

	// Build the full path: ["user", "profile", "name"]
	fullPath := []string{baseKey}
	for _, bracketMatch := range bracketMatches {
		fullPath = append(fullPath, bracketMatch[1])
	}

	// Special case for simple arrays like tags[]
	if len(fullPath) == 2 && fullPath[1] == "" {
		arrayKey := fullPath[0]
		if data[arrayKey] == nil {
			data[arrayKey] = []any{}
		}
		if arr, ok := data[arrayKey].([]any); ok {
			data[arrayKey] = append(arr, value)
		} else {
			data[arrayKey] = []any{value}
		}
		return nil
	}

	// Navigate and create structure for complex paths
	current := data
	for i, pathKey := range fullPath {
		if i == len(fullPath)-1 {
			// Last key - set the value
			current[pathKey] = value
		} else {
			// Intermediate key - ensure structure exists
			if pathKey == "" {
				return fmt.Errorf("empty key not allowed in intermediate path")
			}

			if current[pathKey] == nil {
				current[pathKey] = make(map[string]any)
			}

			// Navigate to next level
			if obj, ok := current[pathKey].(map[string]any); ok {
				current = obj
			} else {
				return fmt.Errorf("cannot navigate through non-object at key: %s", pathKey)
			}
		}
	}

	return nil
}

// MustParseArgumentsAsMap is like ParseArgumentsAsMap but panics on error
func MustParseArgumentsAsMap(args []string) map[string]any {
	result, err := ParseArgumentsAsMap(args)
	if err != nil {
		panic(err)
	}
	return result
}
