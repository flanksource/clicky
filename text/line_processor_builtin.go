package text

import (
	"regexp"
	"strings"
)

// RedactSecrets returns a LineProcessor that redacts sensitive data from lines.
// Uses a tokenizer to properly handle quoted values, ANSI sequences, and complex formats.
// If patterns are provided, they are used as regex patterns (legacy behavior).
//
// The processor replaces only the value portion with "***", keeping the key.
// It never skips lines (always returns skip=false).
//
// Example:
//
//	processor := text.RedactSecrets()
//	result, _ := processor("password=secret123")
//	// result: "password=***"
//
//	result, _ = processor("ALTER USER postgres PASSWORD 'secret'")
//	// result: "ALTER USER postgres PASSWORD '***'"
func RedactSecrets(patterns ...string) LineProcessor {
	// Use tokenizer if no custom patterns provided
	if len(patterns) == 0 {
		return func(line string) (string, bool) {
			tokens := TokenizeLine(line)
			if len(tokens) == 0 {
				return line, false
			}

			// For ANSI-wrapped lines, do simple string replacement
			if strings.Contains(line, "\x1b[") {
				result := line
				for _, token := range tokens {
					// Replace the actual secret value with ***
					switch token.QuoteChar {
					case "'":
						result = strings.ReplaceAll(result, "'"+token.Value+"'", "'***'")
					case "\"":
						result = strings.ReplaceAll(result, "\""+token.Value+"\"", "\"***\"")
					default:
						result = strings.ReplaceAll(result, token.Value, "***")
					}
				}
				return result, false
			}

			// Non-ANSI: use proper tokenizer rebuild
			redacted := make([]Token, 0, len(tokens))
			for _, token := range tokens {
				redacted = append(redacted, Token{
					Key:       token.Key,
					Separator: token.Separator,
					Value:     "***",
					QuoteChar: token.QuoteChar,
					ANSICode:  token.ANSICode,
				})
			}

			result := RebuildLine(line, redacted)
			return result, false
		}
	}

	// Compile custom patterns - these replace entire match with ***
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		if re, err := regexp.Compile(pattern); err == nil {
			compiled = append(compiled, re)
		}
	}

	return func(line string) (string, bool) {
		result := line
		modified := false

		for _, re := range compiled {
			if re.MatchString(result) {
				result = re.ReplaceAllString(result, "***")
				modified = true
			}
		}

		if !modified {
			return line, false
		}
		return result, false
	}
}

// RedactValues returns a LineProcessor that redacts specific known secret values.
// This is useful when you know the actual secret value and want to redact it
// wherever it appears, regardless of the key name.
//
// The processor replaces all occurrences of the specified values with "***".
// It never skips lines (always returns skip=false).
//
// Example:
//
//	processor := text.RedactValues("secret123", "token456")
//	result, _ := processor("password=secret123 token=token456")
//	// result: "password=*** token=***"
func RedactValues(values ...string) LineProcessor {
	// Pre-compile patterns for each value (escape special regex chars)
	patterns := make([]*regexp.Regexp, 0, len(values))
	for _, value := range values {
		if value != "" {
			escaped := regexp.QuoteMeta(value)
			patterns = append(patterns, regexp.MustCompile(escaped))
		}
	}

	return func(line string) (string, bool) {
		result := line
		modified := false

		for _, pattern := range patterns {
			if pattern.MatchString(result) {
				result = pattern.ReplaceAllString(result, "***")
				modified = true
			}
		}

		if !modified {
			return line, false
		}
		return result, false
	}
}

// RegexFilter returns a LineProcessor that filters lines based on a regex pattern.
// If invert=false, lines matching the pattern are skipped.
// If invert=true, lines NOT matching the pattern are skipped.
//
// The processor never modifies the line content.
//
// Example:
//
//	// Skip health check logs
//	processor := clicky.RegexFilter("healthcheck", false)
//
//	// Only keep ERROR logs
//	processor := clicky.RegexFilter("ERROR", true)
func RegexFilter(pattern string, invert bool) LineProcessor {
	re := regexp.MustCompile(pattern)

	return func(line string) (string, bool) {
		matches := re.MatchString(line)

		// If invert=false: skip if matches
		// If invert=true: skip if NOT matches
		skip := matches != invert

		return line, skip
	}
}

// AddPrefix returns a LineProcessor that prepends a prefix to each line.
// The processor never skips lines.
//
// Example:
//
//	processor := clicky.AddPrefix("[INFO] ")
//	result, _ := processor("message")
//	// result: "[INFO] message"
func AddPrefix(prefix string) LineProcessor {
	return func(line string) (string, bool) {
		return prefix + line, false
	}
}

// AddSuffix returns a LineProcessor that appends a suffix to each line.
// The processor never skips lines.
//
// Example:
//
//	processor := clicky.AddSuffix(" [END]")
//	result, _ := processor("message")
//	// result: "message [END]"
func AddSuffix(suffix string) LineProcessor {
	return func(line string) (string, bool) {
		return line + suffix, false
	}
}

// StripSecrets is an alias for RedactSecrets for backward compatibility.
func StripSecrets(patterns ...string) LineProcessor {
	return RedactSecrets(patterns...)
}

// FilterLines returns a LineProcessor that skips lines containing any of the specified strings.
//
// Example:
//
//	// Skip health check and metrics logs
//	processor := clicky.FilterLines("healthcheck", "metrics")
func FilterLines(matches ...string) LineProcessor {
	return func(line string) (string, bool) {
		for _, match := range matches {
			if strings.Contains(line, match) {
				return line, true
			}
		}
		return line, false
	}
}
