package formatters

import "sync"

// BeforeFormatFunc can transform the input before it is formatted.
// ctx is nil for non-request formatting paths and may be an *http.Request,
// echo.Context, or another caller-provided context object.
type BeforeFormatFunc func(ctx any, manager *FormatManager, options FormatOptions, before any) any

// AfterFormatFunc can transform formatted output after rendering.
// before is the value that was passed into the formatter after BeforeFormat
// callbacks were applied.
type AfterFormatFunc func(ctx any, manager *FormatManager, options FormatOptions, before any, out string) string

// FormatCallback registers optional before/after hooks around formatting.
type FormatCallback struct {
	BeforeFormat BeforeFormatFunc
	AfterFormat  AfterFormatFunc
}

var (
	formatCallbacks   []FormatCallback
	formatCallbacksMu sync.RWMutex
)

// AddFormatCallback registers a callback that is applied to subsequent
// formatting operations.
func AddFormatCallback(callback FormatCallback) {
	formatCallbacksMu.Lock()
	defer formatCallbacksMu.Unlock()
	formatCallbacks = append(formatCallbacks, callback)
}

// ClearFormatCallbacks removes all registered format callbacks.
// This is primarily useful in tests.
func ClearFormatCallbacks() {
	formatCallbacksMu.Lock()
	defer formatCallbacksMu.Unlock()
	formatCallbacks = nil
}

func snapshotFormatCallbacks() []FormatCallback {
	formatCallbacksMu.RLock()
	defer formatCallbacksMu.RUnlock()
	if len(formatCallbacks) == 0 {
		return nil
	}
	callbacks := make([]FormatCallback, len(formatCallbacks))
	copy(callbacks, formatCallbacks)
	return callbacks
}

func collapseFormatInput(data ...any) any {
	if len(data) == 1 {
		return data[0]
	}
	values := make([]any, len(data))
	copy(values, data)
	return values
}

func expandFormatInput(data any) []any {
	if values, ok := data.([]any); ok {
		return values
	}
	return []any{data}
}

func (f *FormatManager) applyBeforeFormatCallbacks(ctx any, options FormatOptions, before any) any {
	for _, callback := range snapshotFormatCallbacks() {
		if callback.BeforeFormat != nil {
			before = callback.BeforeFormat(ctx, f, options, before)
		}
	}
	return before
}

func (f *FormatManager) applyAfterFormatCallbacks(ctx any, options FormatOptions, before any, out string) string {
	for _, callback := range snapshotFormatCallbacks() {
		if callback.AfterFormat != nil {
			out = callback.AfterFormat(ctx, f, options, before, out)
		}
	}
	return out
}
