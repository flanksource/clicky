package clicky

import (
	"fmt"
	"io"
	"os"

	"github.com/flanksource/commons/logger"
)

// loggerOutput returns the currently-active logger output, falling back to
// os.Stderr if the logger has no writer configured.
func loggerOutput() io.Writer {
	if out := logger.GetOutput(); out != nil {
		return out
	}
	return os.Stderr
}

// Println writes args separated by spaces and terminated by a newline to the
// currently-active logger output. While a task renderer is active that
// destination is the renderer's serializer, so the line is interleaved
// cleanly with progress frames. Off-renderer it falls through to os.Stderr.
//
// Prefer this (or Printf / Fprintln) over bare fmt.Println / fmt.Fprintln
// on os.Stdout or os.Stderr in library code: direct writes bypass the
// renderer's tracking and leave stale frame lines stacked in the output.
// The clickylint rule `direct-stdout-stderr` flags the bypasses.
func Println(args ...any) {
	_, _ = fmt.Fprintln(loggerOutput(), args...)
}

// Printf formats and writes to the currently-active logger output. See
// Println for the serialization guarantees.
func Printf(format string, args ...any) {
	_, _ = fmt.Fprintf(loggerOutput(), format, args...)
}

// Fprintln writes args separated by spaces and terminated by a newline to
// the supplied writer. Use this when you need to target a specific writer
// (e.g. a tab writer, a string builder, a capture buffer) — for the common
// case of "print a line to the terminal", prefer Println.
func Fprintln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}
