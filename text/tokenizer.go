package text

import (
	"strings"
)

// Token represents a parsed key-value pair with quote information
type Token struct {
	Key            string // The keyword (password, token, etc.)
	Separator      string // The separator (=, :, or space)
	Value          string // The actual value (unescaped)
	QuoteChar      string // Quote type: ', ", ansi, or empty
	ANSICode       string // The ANSI escape code if QuoteChar is "ansi"
	InnerQuoteChar string // Quote char inside ANSI (for \x1b[37m'value'\x1b[0m)
	StartPos       int    // Starting position in original line
	EndPos         int    // Ending position in original line
}

// Known secret keywords (case-insensitive matching)
var secretKeywords = []string{
	"password", "passwd", "pwd", "pass",
	"token", "api_key", "api-key", "apikey",
	"secret", "auth", "bearer", "authorization",
}

// StripANSI removes ANSI escape codes from a string. It handles CSI
// sequences (\x1b[...) terminated by any byte in 0x40-0x7E, OSC sequences
// (\x1b]...) terminated by BEL or ST (\x1b\\), and standalone two-byte
// escapes (\x1b X for X in 0x40-0x5F).
func StripANSI(s string) string {
	var result strings.Builder
	result.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] != 0x1b || i+1 >= len(s) {
			result.WriteByte(s[i])
			i++
			continue
		}
		switch s[i+1] {
		case '[':
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7E) {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
		case ']':
			j := i + 2
			for j < len(s) {
				if s[j] == 0x07 {
					j++
					break
				}
				if s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
					j += 2
					break
				}
				j++
			}
			i = j
		default:
			i += 2
		}
	}
	return result.String()
}

// TokenizeLine parses a line and extracts secret key-value tokens.
// If the line contains ANSI escape sequences, they are stripped before parsing.
func TokenizeLine(line string) []Token {
	// Check if line has ANSI codes
	hasANSI := strings.Contains(line, "\x1b[")
	workingLine := line
	if hasANSI {
		workingLine = StripANSI(line)
	}

	var tokens []Token
	i := 0
	length := len(workingLine)

	for i < length {
		// Find next keyword in stripped version
		keyword, keyStart, keyEnd := findNextKeyword(workingLine, i)
		if keyword == "" {
			break
		}

		// Move to end of keyword
		i = keyEnd

		// Find separator
		separator, sepEnd := findSeparator(workingLine, i)
		if separator == "" {
			break
		}
		i = sepEnd

		// Parse value (with quote handling)
		value, quoteChar, ansiCode, innerQuote, valueEnd := parseValue(workingLine, i, length)

		// Map positions back to original line if needed
		actualKeyStart := keyStart
		actualEndPos := valueEnd
		if hasANSI {
			// For ANSI lines, positions in workingLine don't match original
			// We'll use the stripped version for matching but original for rebuild
			actualKeyStart = 0
			actualEndPos = len(line)
		}

		tokens = append(tokens, Token{
			Key:            keyword,
			Separator:      separator,
			Value:          value,
			QuoteChar:      quoteChar,
			ANSICode:       ansiCode,
			InnerQuoteChar: innerQuote,
			StartPos:       actualKeyStart,
			EndPos:         actualEndPos,
		})

		i = valueEnd
	}

	return tokens
}

// findNextKeyword searches for the next secret keyword starting from position start
func findNextKeyword(line string, start int) (keyword string, keyStart, keyEnd int) {
	lower := strings.ToLower(line[start:])

	for pos := 0; pos < len(lower); pos++ {
		for _, kw := range secretKeywords {
			if pos+len(kw) <= len(lower) {
				substr := lower[pos : pos+len(kw)]
				if substr == kw {
					// Check it's a word boundary (not part of larger word)
					if pos+len(kw) < len(lower) {
						nextChar := lower[pos+len(kw)]
						if nextChar != ' ' && nextChar != '=' && nextChar != ':' && nextChar != '\t' {
							continue
						}
					}

					// Return original case keyword
					return line[start+pos : start+pos+len(kw)], start + pos, start + pos + len(kw)
				}
			}
		}
	}

	return "", -1, -1
}

// findSeparator finds the separator (=, :, or space) after a keyword
func findSeparator(line string, start int) (separator string, end int) {
	if start >= len(line) {
		return "", start
	}

	// Skip whitespace
	i := start
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}

	if i >= len(line) {
		return "", i
	}

	// Check for = or :
	if line[i] == '=' {
		i++
		// Skip trailing spaces after =
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		return "=", i
	}

	if line[i] == ':' {
		i++
		// Skip trailing spaces after :
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		return ": ", i
	}

	// Must be space separator
	if start < i {
		return " ", i
	}

	return "", start
}

// parseValue extracts the value, handling quotes and escapes
// Returns: value, quoteChar, ansiCode, innerQuoteChar, end
func parseValue(line string, start, length int) (value, quoteChar, ansiCode, innerQuote string, end int) {
	if start >= length {
		return "", "", "", "", start
	}

	// Check for ANSI sequence
	if start+2 < length && line[start:start+2] == "\x1b[" {
		return parseANSIValue(line, start, length)
	}

	// Check for quotes
	if line[start] == '\'' {
		v, q, a, e := parseQuotedValue(line, start, length, '\'')
		return v, q, a, "", e
	}
	if line[start] == '"' {
		v, q, a, e := parseQuotedValue(line, start, length, '"')
		return v, q, a, "", e
	}

	// Unquoted value - consume until next keyword or EOL
	v, q, a, e := parseUnquotedValue(line, start, length)
	return v, q, a, "", e
}

// parseANSIValue parses a value wrapped in ANSI escape sequences
// Handles both: \x1b[36mvalue\x1b[0m and \x1b[37m'value'\x1b[0m
// Returns: value, quoteChar, ansiCode, innerQuoteChar, end
func parseANSIValue(line string, start, length int) (value, quoteChar, ansiCode, innerQuote string, end int) {
	// Find the 'm' that ends the ANSI code
	codeEnd := strings.Index(line[start:], "m")
	if codeEnd == -1 {
		// No valid ANSI sequence
		v, q, a, e := parseUnquotedValue(line, start, length)
		return v, q, a, "", e
	}
	codeEnd += start + 1
	ansiCode = line[start:codeEnd]

	// Find the closing \x1b[0m
	resetSeq := "\x1b[0m"
	resetPos := strings.Index(line[codeEnd:], resetSeq)
	if resetPos == -1 {
		// No reset, consume to EOL
		value = line[codeEnd:]
		return value, "ansi", ansiCode, "", length
	}

	content := line[codeEnd : codeEnd+resetPos]

	// Check if content is a quoted value: 'value' or "value"
	if len(content) >= 2 && (content[0] == '\'' || content[0] == '"') {
		quote := content[0]
		if content[len(content)-1] == quote {
			// ANSI wraps a quoted value: \x1b[37m'value'\x1b[0m
			value = content[1 : len(content)-1] // Strip outer quotes
			return value, "ansi", ansiCode, string(quote), codeEnd + resetPos + len(resetSeq)
		}
	}

	// ANSI wraps unquoted value: \x1b[36mvalue\x1b[0m
	value = content
	return value, "ansi", ansiCode, "", codeEnd + resetPos + len(resetSeq)
}

// parseQuotedValue parses a quoted value with escape handling
func parseQuotedValue(line string, start, length int, quote rune) (value, quoteChar, ansiCode string, end int) {
	i := start + 1 // Skip opening quote
	var buf strings.Builder

	for i < length {
		ch := line[i]

		// Check for escape sequence
		if ch == '\\' && i+1 < length {
			nextCh := line[i+1]
			if nextCh == '\'' || nextCh == '"' || nextCh == '\\' {
				buf.WriteByte(nextCh)
				i += 2
				continue
			}
		}

		// Check for closing quote
		if rune(ch) == quote {
			return buf.String(), string(quote), "", i + 1
		}

		buf.WriteByte(ch)
		i++
	}

	// Unclosed quote - return what we have
	return buf.String(), string(quote), "", length
}

// parseUnquotedValue parses an unquoted value until next keyword or EOL
func parseUnquotedValue(line string, start, length int) (value, quoteChar, ansiCode string, end int) {
	// Consume until we hit another keyword or EOL
	i := start
	valueEnd := start

	for i < length {
		ch := line[i]

		// Check if we're at the start of another keyword (skip leading spaces)
		if ch == ' ' || ch == '\t' {
			// Skip whitespace
			j := i
			for j < length && (line[j] == ' ' || line[j] == '\t') {
				j++
			}

			// Check if a keyword starts after the whitespace
			for _, kw := range secretKeywords {
				if j+len(kw) <= length {
					substr := strings.ToLower(line[j : j+len(kw)])
					if substr == kw {
						// Check word boundary
						if j+len(kw) < length {
							nextChar := line[j+len(kw)]
							if nextChar == ' ' || nextChar == '=' || nextChar == ':' || nextChar == '\t' {
								// Found next keyword, stop before the whitespace
								value = strings.TrimRight(line[start:i], " \t")
								return value, "", "", i
							}
						} else {
							// Keyword at EOL
							value = strings.TrimRight(line[start:i], " \t")
							return value, "", "", i
						}
					}
				}
			}
		}

		valueEnd = i + 1
		i++
	}

	// Reached EOL
	value = strings.TrimRight(line[start:valueEnd], " \t")
	return value, "", "", valueEnd
}

// RebuildLine reconstructs a line by replacing token values with redacted versions.
// It tokenizes the original line, matches against provided tokens, and rebuilds.
func RebuildLine(original string, redactedTokens []Token) string {
	if len(redactedTokens) == 0 {
		return original
	}

	// Tokenize original to get positions
	originalTokens := TokenizeLine(original)
	if len(originalTokens) == 0 {
		return original
	}

	// Build map of tokens to redact (by key)
	redactions := make(map[string]Token)
	for _, rt := range redactedTokens {
		redactions[strings.ToLower(rt.Key)] = rt
	}

	// Rebuild line
	var result strings.Builder
	lastEnd := 0

	for _, token := range originalTokens {
		// Check if this token should be redacted
		redacted, shouldRedact := redactions[strings.ToLower(token.Key)]

		// Copy non-tokenized portion before this token
		if token.StartPos > lastEnd {
			result.WriteString(original[lastEnd:token.StartPos])
		}

		if shouldRedact {
			// Write redacted version
			result.WriteString(token.Key)
			result.WriteString(token.Separator)

			if token.QuoteChar == "ansi" {
				result.WriteString(token.ANSICode)
				if token.InnerQuoteChar != "" {
					// ANSI with inner quotes: \x1b[37m'***'\x1b[0m
					result.WriteString(token.InnerQuoteChar)
					result.WriteString(redacted.Value)
					result.WriteString(token.InnerQuoteChar)
				} else {
					// ANSI without quotes: \x1b[36m***\x1b[0m
					result.WriteString(redacted.Value)
				}
				result.WriteString("\x1b[0m")
			} else if token.QuoteChar != "" {
				result.WriteString(token.QuoteChar)
				result.WriteString(redacted.Value)
				result.WriteString(token.QuoteChar)
			} else {
				result.WriteString(redacted.Value)
			}
		} else {
			// Keep original token
			result.WriteString(original[token.StartPos:token.EndPos])
		}

		lastEnd = token.EndPos
	}

	// Copy remaining non-tokenized portion
	if lastEnd < len(original) {
		result.WriteString(original[lastEnd:])
	}

	return result.String()
}
