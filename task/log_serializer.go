package task

import (
	"bytes"
	"io"
	"sync"
)

// logSerializingWriter is installed as flanksource/commons/logger's output
// writer while a task renderer owns the terminal. It solves two problems:
//
//  1. Interleaving — every Write holds the manager's output mutex so log
//     lines cannot land mid-frame between a ClearLines and the fresh
//     content that follows.
//  2. Line accounting — the renderer's ClearLines(lastLines) clears the
//     region occupied by the previous tick's content. When a log line
//     lands between ticks, the cursor advances past the tracked region
//     and the next tick's ClearLines undercounts, leaving the top of the
//     previous frame stacked above the new one. We count newlines we
//     write so the next tick can add them to lastLines.
//
// The writer delegates to `next` — whatever writer the logger was targeting
// before the renderer took over (normally os.Stderr, but a caller-installed
// wrapper is equally fine). Delegating preserves the caller's logging
// pipeline; we only add ordering and accounting, never change destination.
type logSerializingWriter struct {
	mu   *sync.Mutex
	next io.Writer

	// linesWritten counts newlines emitted through this writer since the
	// last TakeLinesWritten call. The next interactiveRender adds this to
	// its lastLines input so ClearLines covers the log lines too. Access
	// only while holding mu.
	linesWritten int
}

func newLogSerializingWriter(mu *sync.Mutex, next io.Writer) *logSerializingWriter {
	if mu == nil {
		mu = &sync.Mutex{}
	}
	return &logSerializingWriter{mu: mu, next: next}
}

// Write serializes one log message against concurrent tick renders on the
// same underlying mutex. Short holds: a single Write of the fully-formatted
// log line. No re-entry into the renderer.
func (l *logSerializingWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.next == nil {
		return len(p), nil
	}
	n, err := l.next.Write(p)
	l.linesWritten += bytes.Count(p[:n], []byte{'\n'})
	return n, err
}

// TakeLinesWritten returns the accumulated newline count and resets it to
// zero. The renderer calls this under mu immediately before ClearLines so
// the cleared region covers log lines emitted since the last tick. Caller
// must hold mu (interactiveRender does).
func (l *logSerializingWriter) TakeLinesWritten() int {
	n := l.linesWritten
	l.linesWritten = 0
	return n
}
