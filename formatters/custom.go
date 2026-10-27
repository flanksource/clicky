package formatters

import (
	"mime"
	"sort"
	"strings"
	"sync"
)

// FormatterFunc is a custom formatter function that converts data to a formatted string.
// It receives the data to format and format options, returning the formatted output or an error.
type FormatterFunc func(data interface{}, options FormatOptions) (string, error)

// RegisterOption configures a custom formatter registration.
type RegisterOption func(*customFormatter)

type customFormatter struct {
	fn          FormatterFunc
	contentType string
}

// WithContentType declares the media type a custom format produces, e.g.
// "application/pdf". HTTP responses in that format carry it as Content-Type,
// and a non-text type marks the output binary so it is never ANSI-stripped.
func WithContentType(contentType string) RegisterOption {
	return func(f *customFormatter) { f.contentType = contentType }
}

var (
	customFormatters = make(map[string]customFormatter)
	formattersMutex  sync.RWMutex
)

// RegisterFormatter registers a custom formatter with the given name.
// The name is case-insensitive and will be normalized to lowercase.
// Custom formatters take precedence over built-in formatters with the same name.
//
// Example:
//
//	RegisterFormatter("upper", func(data interface{}, opts FormatOptions) (string, error) {
//	    return strings.ToUpper(fmt.Sprintf("%v", data)), nil
//	})
func RegisterFormatter(name string, fn FormatterFunc, opts ...RegisterOption) {
	formatter := customFormatter{fn: fn}
	for _, opt := range opts {
		opt(&formatter)
	}
	formattersMutex.Lock()
	defer formattersMutex.Unlock()
	customFormatters[strings.ToLower(name)] = formatter
}

// GetCustomFormatter retrieves a custom formatter by name.
// Returns the formatter function and true if found, nil and false otherwise.
// The name lookup is case-insensitive.
func GetCustomFormatter(name string) (FormatterFunc, bool) {
	formattersMutex.RLock()
	defer formattersMutex.RUnlock()
	formatter, exists := customFormatters[strings.ToLower(name)]
	return formatter.fn, exists
}

// CustomFormatterContentType is the media type a custom format was registered
// with (WithContentType); false when the format is not registered or declared none.
func CustomFormatterContentType(name string) (string, bool) {
	formattersMutex.RLock()
	defer formattersMutex.RUnlock()
	formatter, exists := customFormatters[strings.ToLower(name)]
	return formatter.contentType, exists && formatter.contentType != ""
}

// isBinaryContentType reports whether a declared media type carries bytes
// rather than text: anything outside text/* and the JSON/XML/YAML families.
func isBinaryContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.ToLower(strings.TrimSpace(contentType))
	}
	if strings.HasPrefix(mediaType, "text/") {
		return false
	}
	for _, textual := range []string{"json", "xml", "yaml", "javascript"} {
		if strings.Contains(mediaType, textual) {
			return false
		}
	}
	return true
}

// ListCustomFormatters returns a sorted list of all registered custom formatter names.
func ListCustomFormatters() []string {
	formattersMutex.RLock()
	defer formattersMutex.RUnlock()
	names := make([]string, 0, len(customFormatters))
	for name := range customFormatters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// UnregisterFormatter removes a custom formatter by name.
// This is primarily useful for testing.
func UnregisterFormatter(name string) {
	formattersMutex.Lock()
	defer formattersMutex.Unlock()
	delete(customFormatters, strings.ToLower(name))
}

// ClearCustomFormatters removes all registered custom formatters.
// This is primarily useful for testing.
func ClearCustomFormatters() {
	formattersMutex.Lock()
	defer formattersMutex.Unlock()
	customFormatters = make(map[string]customFormatter)
}
