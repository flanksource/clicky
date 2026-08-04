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
//  2. Persistence — in interactive mode (buffered), log lines written
//     between ticks are held in pending and emitted by the next tick after
//     it clears the old frame and before it paints the new one, so they
//     persist in scrollback above the live frame (the bubbletea
//     tea.Println pattern) instead of being erased with the frame.
//
// The writer delegates to `next` — whatever writer the logger was targeting
// before the renderer took over (normally os.Stderr, but a caller-installed
// wrapper is equally fine). Delegating preserves the caller's logging
// pipeline; we only add ordering and positioning, never change destination.
type logSerializingWriter struct {
	mu   *sync.Mutex
	next io.Writer

	// buffered is true in interactive render mode: writes accumulate in
	// pending until the next tick's FlushPending emits them above the
	// repainted frame. In plain mode writes pass straight through to next.
	buffered bool

	// pending holds log bytes awaiting the next tick's FlushPending.
	// Access only while holding mu.
	pending bytes.Buffer
}

func newLogSerializingWriter(mu *sync.Mutex, next io.Writer, buffered bool) *logSerializingWriter {
	if mu == nil {
		mu = &sync.Mutex{}
	}
	return &logSerializingWriter{mu: mu, next: next, buffered: buffered}
}

// Write serializes one log message against concurrent tick renders on the
// same underlying mutex. Interactive mode buffers the bytes for the next
// tick; plain mode delegates immediately. Short holds, no re-entry into
// the renderer.
func (l *logSerializingWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buffered {
		l.pending.Write(p)
		return len(p), nil
	}
	if l.next == nil {
		return len(p), nil
	}
	return l.next.Write(p)
}

// FlushPending writes buffered log bytes to the underlying writer, ensuring
// they end on a fresh line so the frame repainted after them starts at
// column 0. The renderer calls this between clearing the old frame and
// painting the new one; uninstallLogSerializer calls it so a line that
// landed after the final tick is emitted rather than dropped. Caller must
// hold mu (interactiveRender does).
func (l *logSerializingWriter) FlushPending() {
	if l.pending.Len() == 0 {
		return
	}
	if l.next != nil {
		b := l.pending.Bytes()
		_, _ = l.next.Write(b)
		if b[len(b)-1] != '\n' {
			_, _ = io.WriteString(l.next, "\n")
		}
	}
	l.pending.Reset()
}
