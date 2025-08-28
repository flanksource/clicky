package api

import (
	"strings"
)

// PrettifyFieldName converts field names to readable format
func PrettifyFieldName(name string) string {
	// Convert snake_case and camelCase to Title Case
	var result strings.Builder

	// First try to split on underscores and dashes
	words := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-'
	})

	// If we only got one word (no underscores/dashes), try camelCase splitting
	if len(words) == 1 {
		words = SplitCamelCase(name)
	}

	for i, word := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(strings.Title(strings.ToLower(word)))
	}

	return result.String()
}

// SplitCamelCase splits camelCase strings into words
func SplitCamelCase(s string) []string {
	var words []string
	var current strings.Builder
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Check if this rune starts a new word
		if i > 0 && r >= 'A' && r <= 'Z' {
			// Look back to see if previous character was lowercase
			prevIsLower := i > 0 && runes[i-1] >= 'a' && runes[i-1] <= 'z'

			// Only split on uppercase if previous was lowercase (simple camelCase like firstName, userID)
			// This keeps acronyms together (HTTPRequest stays as one word)
			if prevIsLower {
				if current.Len() > 0 {
					words = append(words, current.String())
					current.Reset()
				}
			}
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}
