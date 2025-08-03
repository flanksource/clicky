package clicky

import (
	"encoding/json"
	"strings"
)

// JSONParser handles lenient JSON parsing
type JSONParser struct{}

// NewJSONParser creates a new JSON parser
func NewJSONParser() *JSONParser {
	return &JSONParser{}
}

// ParseJSON parses JSON in a lenient way, allowing for various formats
func ParseJSON(data []byte) (interface{}, error) {
	var result interface{}

	// First try standard JSON parsing
	if err := json.Unmarshal(data, &result); err == nil {
		return result, nil
	}

	// Try parsing as string (in case it's quoted JSON)
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		// Try parsing the string as JSON
		if err := json.Unmarshal([]byte(str), &result); err == nil {
			return result, nil
		}
		// Return as string if it's not parseable JSON
		return str, nil
	}

	// Try parsing with relaxed rules (remove comments, trailing commas, etc.)
	cleaned := cleanJSONString(string(data))
	if err := json.Unmarshal([]byte(cleaned), &result); err == nil {
		return result, nil
	}

	// As last resort, return the original string
	return string(data), nil
}

// Parse is an alias for ParseJSON for consistency with other parsers
func (p *JSONParser) Parse(data []byte) (interface{}, error) {
	return ParseJSON(data)
}

// cleanJSONString attempts to clean JSON string for parsing
func cleanJSONString(s string) string {
	// Remove single-line comments
	lines := strings.Split(s, "\n")
	var cleaned []string

	for _, line := range lines {
		// Remove // comments
		if idx := strings.Index(line, "//"); idx != -1 {
			line = line[:idx]
		}
		// Remove /* */ comments (simple implementation)
		if start := strings.Index(line, "/*"); start != -1 {
			if end := strings.Index(line[start:], "*/"); end != -1 {
				line = line[:start] + line[start+end+2:]
			}
		}
		// Remove trailing commas before } or ]
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, ",}") {
			line = line[:len(line)-2] + "}"
		}
		if strings.HasSuffix(line, ",]") {
			line = line[:len(line)-2] + "]"
		}
		cleaned = append(cleaned, line)
	}

	return strings.Join(cleaned, "\n")
}
