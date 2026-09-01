package entity

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/flanksource/commons/logger"
)

const truncationMarker = "...[truncated]"

func sanitizeErrorText(value string, limit int) (string, string, bool) {
	value = logger.StripSecrets(value)
	contentType := detectedContentType([]byte(value))
	value, truncated := truncateUTF8(value, limit)
	return value, contentType, truncated
}

func sanitizeErrorDetail(label string, value any, limit int) ErrorDetail {
	raw := diagnosticBytes(value)
	contentType := detectedContentType(raw)
	if !isTextualContentType(contentType) {
		return ErrorDetail{
			Label: label, ContentType: contentType,
			Value: fmt.Sprintf("[binary %s, %d bytes]", contentType, len(raw)), Truncated: true,
		}
	}

	var sanitized string
	switch {
	case json.Valid(raw):
		var body any
		if err := json.Unmarshal(raw, &body); err != nil {
			panic(fmt.Sprintf("decode valid JSON diagnostic: %v", err))
		}
		encoded, err := json.Marshal(sanitizeJSONValue(body, limit))
		if err != nil {
			panic(fmt.Sprintf("encode sanitized JSON diagnostic: %v", err))
		}
		sanitized = string(encoded)
		contentType = "application/json"
	case looksLikeForm(raw):
		values, err := url.ParseQuery(string(raw))
		if err == nil {
			for key, entries := range values {
				for i, entry := range entries {
					if logger.IsSensitiveKey(key) {
						entries[i] = logger.PrintableSecret(entry)
					} else {
						entries[i] = logger.StripSecrets(entry)
					}
				}
				values[key] = entries
			}
			sanitized = values.Encode()
			contentType = "application/x-www-form-urlencoded"
		} else {
			sanitized = logger.StripSecrets(string(raw))
		}
	default:
		sanitized = logger.StripSecrets(string(raw))
	}
	sanitized, truncated := truncateUTF8(sanitized, limit)
	return ErrorDetail{Label: label, Value: sanitized, ContentType: contentType, Truncated: truncated}
}

func sanitizeDiagnosticValue(key string, value any, limit int) any {
	if logger.IsSensitiveKey(key) {
		return logger.PrintableSecret(fmt.Sprint(value))
	}
	switch typed := value.(type) {
	case string:
		return logger.StripSecrets(typed)
	case []byte:
		return sanitizeErrorDetail(key, typed, limit).Value
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for nestedKey, nestedValue := range typed {
			clean[nestedKey] = sanitizeDiagnosticValue(nestedKey, nestedValue, limit)
		}
		return clean
	case map[string]string:
		clean := make(map[string]any, len(typed))
		for nestedKey, nestedValue := range typed {
			clean[nestedKey] = sanitizeDiagnosticValue(nestedKey, nestedValue, limit)
		}
		return clean
	case []any:
		clean := make([]any, len(typed))
		for i, nestedValue := range typed {
			clean[i] = sanitizeDiagnosticValue("", nestedValue, limit)
		}
		return clean
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return logger.StripSecrets(fmt.Sprint(value))
		}
		var normalized any
		if err := json.Unmarshal(encoded, &normalized); err != nil {
			return logger.StripSecrets(fmt.Sprint(value))
		}
		return sanitizeJSONValue(normalized, limit)
	}
}

func sanitizeJSONValue(value any, limit int) any {
	switch typed := value.(type) {
	case string:
		// A root JSON string or a string inside a JSON array carries the same
		// exposure risk as an object field value: strip secrets before it is
		// re-encoded into an error detail.
		return logger.StripSecrets(typed)
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, nested := range typed {
			clean[key] = sanitizeDiagnosticValue(key, nested, limit)
		}
		return clean
	case []any:
		for i, nested := range typed {
			typed[i] = sanitizeJSONValue(nested, limit)
		}
		return typed
	default:
		return value
	}
}

func diagnosticBytes(value any) []byte {
	switch typed := value.(type) {
	case []byte:
		return typed
	case string:
		return []byte(typed)
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return []byte(fmt.Sprint(typed))
		}
		return encoded
	}
}

func isBodyDiagnostic(key string, value any) bool {
	lower := strings.ToLower(key)
	if strings.Contains(lower, "body") || strings.Contains(lower, "payload") || strings.Contains(lower, "response") {
		return true
	}
	_, binary := value.([]byte)
	return binary
}

func detectedContentType(value []byte) string {
	if json.Valid(value) {
		return "application/json"
	}
	if looksLikeForm(value) {
		return "application/x-www-form-urlencoded"
	}
	contentType := http.DetectContentType(value)
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		return mediaType
	}
	return contentType
}

func isTextualContentType(contentType string) bool {
	return strings.HasPrefix(contentType, "text/") ||
		strings.Contains(contentType, "json") ||
		strings.Contains(contentType, "xml") ||
		strings.Contains(contentType, "yaml") ||
		contentType == "application/x-www-form-urlencoded"
}

func looksLikeForm(value []byte) bool {
	if len(value) == 0 || !utf8.Valid(value) || !strings.Contains(string(value), "=") {
		return false
	}
	for _, field := range strings.Split(string(value), "&") {
		key, _, found := strings.Cut(field, "=")
		if !found || key == "" || strings.IndexFunc(key, unicode.IsSpace) >= 0 {
			return false
		}
	}
	parsed, err := url.ParseQuery(string(value))
	return err == nil && len(parsed) > 0
}

func truncateUTF8(value string, limit int) (string, bool) {
	if limit <= 0 || len(value) <= limit {
		return value, false
	}
	if limit <= len(truncationMarker) {
		return truncationMarker[:limit], true
	}
	// Only the cut boundary needs adjusting: back up while the cut would split
	// a multi-byte rune. Validating the whole prefix would discard all content
	// for input that is invalid UTF-8 anywhere before the cut, and would make
	// truncation a quadratic scan.
	end := limit - len(truncationMarker)
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end] + truncationMarker, true
}
