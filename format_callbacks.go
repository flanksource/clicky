package clicky

import (
	"fmt"

	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/task"
)

type FormatManager = formatters.FormatManager
type FormatCallback = formatters.FormatCallback
type BeforeFormatFunc = formatters.BeforeFormatFunc
type AfterFormatFunc = formatters.AfterFormatFunc

// AddFormatCallback registers a global formatting callback. The callback is
// applied to top-level clicky formatting as well as HTTP/RPC response
// formatting paths.
func AddFormatCallback(callback FormatCallback) {
	formatters.AddFormatCallback(callback)
}

// ClearFormatCallbacks removes all registered format callbacks.
// This is primarily useful in tests.
func ClearFormatCallbacks() {
	formatters.ClearFormatCallbacks()
}

// FormatWithContext formats using the shared clicky formatter while forwarding
// ctx to registered format callbacks.
func FormatWithContext(ctx any, o any, opts ...FormatOptions) (string, error) {
	return Formatter.FormatWithContext(ctx, formatters.MergeOptions(append([]FormatOptions{defaultOpts}, opts...)...), o)
}

// MustFormatWithContext formats using the shared clicky formatter and panics on
// error.
func MustFormatWithContext(ctx any, o any, opts ...FormatOptions) string {
	result, _ := Formatter.FormatWithContext(ctx, formatters.MergeOptions(append([]FormatOptions{defaultOpts}, opts...)...), o)
	return result
}

// MustPrintWithContext formats using the shared clicky formatter, writes the
// output to stdout, and forwards ctx to registered format callbacks.
func MustPrintWithContext(ctx any, o any, opts ...FormatOptions) {
	_ = task.Wait()
	result, err := FormatWithContext(ctx, o, opts...)
	if err != nil {
		panic(err)
	}

	fmt.Print(result)
}

// FormatToFileWithContext formats using the shared clicky formatter, writes the
// output to file, and forwards ctx to registered format callbacks.
func FormatToFileWithContext(ctx any, o any, opts FormatOptions, file string) error {
	opts.Output = file
	_opts := formatters.MergeOptions(append([]FormatOptions{defaultOpts}, opts)...)
	return Formatter.FormatToFileWithContext(ctx, _opts, o)
}
