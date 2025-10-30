package text

import (
	"regexp"
	"strings"
)

// secretPattern defines a regex pattern with a replacement string
type secretPattern struct {
	pattern     *regexp.Regexp
	replacement string
}

// buildSecretPatterns creates patterns for detecting secrets with different quote styles
func buildSecretPatterns() []secretPattern {
	keywords := []string{
		"password|passwd|pwd|pass",
		"token|api[_-]?key|apikey|secret|auth",
		"bearer|authorization",
	}

	var patterns []secretPattern
	for _, kw := range keywords {
		// Pattern 1: key='value' (single quotes)
		patterns = append(patterns, secretPattern{
			pattern:     regexp.MustCompile(`(?i)(` + kw + `)(\s*[=:]\s*|[ \t]+)('([^']*)')`),
			replacement: "${1}${2}'***'",
		})
		// Pattern 2: key="value" (double quotes)
		patterns = append(patterns, secretPattern{
			pattern:     regexp.MustCompile(`(?i)(` + kw + `)(\s*[=:]\s*|[ \t]+)("([^"]*)")`),
			replacement: `${1}${2}"***"`,
		})
		// Pattern 3: key=value or key: value (no quotes) - exclude leading quotes
		patterns = append(patterns, secretPattern{
			pattern:     regexp.MustCompile(`(?i)(` + kw + `)(\s*[=:]\s*)([^'"\s]\S*)`),
			replacement: "${1}${2}***",
		})
	}
	return patterns
}

var defaultSecretPatterns = buildSecretPatterns()

// RedactSecrets returns a LineProcessor that redacts sensitive data from lines.
// If patterns are provided, they are used as regex patterns to match secrets.
// If no patterns are provided, default patterns for common secrets are used.
//
// The processor replaces only the value portion with "***", keeping the key.
// It never skips lines (always returns skip=false).
//
// Example:
//
//	processor := text.RedactSecrets()
//	result, _ := processor("password=secret123")
//	// result: "password=***"
func RedactSecrets(patterns ...string) LineProcessor {
	// Use default patterns if none provided
	if len(patterns) == 0 {
		return func(line string) (string, bool) {
			result := line
			modified := false

			for _, sp := range defaultSecretPatterns {
				if sp.pattern.MatchString(result) {
					result = sp.pattern.ReplaceAllString(result, sp.replacement)
					modified = true
				}
			}

			if !modified {
				return line, false
			}
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
