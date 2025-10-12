package formatters

import (
	"sort"
	"strings"
	"sync"
)

// FormatterFunc is a custom formatter function that converts data to a formatted string.
// It receives the data to format and format options, returning the formatted output or an error.
type FormatterFunc func(data interface{}, options FormatOptions) (string, error)

var (
	customFormatters = make(map[string]FormatterFunc)
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
func RegisterFormatter(name string, fn FormatterFunc) {
	formattersMutex.Lock()
	defer formattersMutex.Unlock()
	customFormatters[strings.ToLower(name)] = fn
}

// GetCustomFormatter retrieves a custom formatter by name.
// Returns the formatter function and true if found, nil and false otherwise.
// The name lookup is case-insensitive.
func GetCustomFormatter(name string) (FormatterFunc, bool) {
	formattersMutex.RLock()
	defer formattersMutex.RUnlock()
	fn, exists := customFormatters[strings.ToLower(name)]
	return fn, exists
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
	customFormatters = make(map[string]FormatterFunc)
}
