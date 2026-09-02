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
	value = logger.StripSecrets(strings.ToValidUTF8(value, "�"))
	contentType := detectedContentType([]byte(value))
	value, truncated := truncateUTF8(value, limit)
	return value, contentType, truncated
}

func sanitizeErrorDetail(label string, value any, limit int) ErrorDetail {
	raw := diagnosticBytes(value)
	contentType := detectedContentType(raw)
	if !isTextualContentType(contentType) {
		message, _ := truncateUTF8(fmt.Sprintf("[binary %s, %d bytes]", contentType, len(raw)), limit)
		return ErrorDetail{
			Label: label, ContentType: contentType, Value: message, Truncated: true,
		}
	}

	var sanitized string
	switch {
	case json.Valid(raw):
		var body any
		if err := json.Unmarshal(raw, &body); err == nil {
			if encoded, marshalErr := json.Marshal(sanitizeJSONValue(body, limit)); marshalErr == nil {
				sanitized = string(encoded)
				contentType = "application/json"
			}
		}
	case looksLikeForm(raw):
		if values, err := url.ParseQuery(string(raw)); err == nil {
			for key, entries := range values {
				for index, entry := range entries {
					entries[index] = sanitizeScalar(key, entry, limit)
				}
				values[key] = entries
			}
			sanitized = values.Encode()
			contentType = "application/x-www-form-urlencoded"
		}
	}
	if sanitized == "" {
		sanitized = logger.StripSecrets(strings.ToValidUTF8(string(raw), "�"))
	}
	sanitized, truncated := truncateUTF8(sanitized, limit)
	return ErrorDetail{Label: label, Value: sanitized, ContentType: contentType, Truncated: truncated}
}

func sanitizeDiagnosticValue(key string, value any, limit int) any {
	switch typed := value.(type) {
	case string:
		return sanitizeScalar(key, typed, limit)
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
		for index, nestedValue := range typed {
			clean[index] = sanitizeDiagnosticValue("", nestedValue, limit)
		}
		return clean
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			text, _, _ := sanitizeErrorText(fmt.Sprint(value), limit)
			return text
		}
		var normalized any
		if err := json.Unmarshal(encoded, &normalized); err != nil {
			text, _, _ := sanitizeErrorText(fmt.Sprint(value), limit)
			return text
		}
		return sanitizeJSONValue(normalized, limit)
	}
}

func sanitizeScalar(key, value string, limit int) string {
	if logger.IsSensitiveKey(key) {
		value = logger.PrintableSecret(value)
	} else {
		value = logger.StripSecrets(value)
	}
	value, _ = truncateUTF8(strings.ToValidUTF8(value, "�"), limit)
	return value
}

func sanitizeJSONValue(value any, limit int) any {
	switch typed := value.(type) {
	case string:
		return sanitizeScalar("", typed, limit)
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, nested := range typed {
			clean[key] = sanitizeDiagnosticValue(key, nested, limit)
		}
		return clean
	case []any:
		clean := make([]any, len(typed))
		for index, nested := range typed {
			clean[index] = sanitizeJSONValue(nested, limit)
		}
		return clean
	default:
		return value
	}
}

func diagnosticBytes(value any) []byte {
	switch typed := value.(type) {
	case []byte:
		return append([]byte(nil), typed...)
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
	value = strings.ToValidUTF8(value, "�")
	if limit <= 0 || len(value) <= limit {
		return value, false
	}
	if limit <= len(truncationMarker) {
		return truncationMarker[:limit], true
	}
	end := limit - len(truncationMarker)
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end] + truncationMarker, true
}
