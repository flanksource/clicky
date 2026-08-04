package task

import (
	"bytes"
	"io"
	"os"
	"sync"
)

// stderrGate forwards writes to os.Stderr unless the interactive task
// renderer currently owns the TTY, in which case the write is buffered and
// flushed to os.Stderr right after the renderer releases the terminal
// (stopRender's teardown calls flushGatedStderr). Used by callers that emit
// log-style content to stderr (e.g. clicky.Infof's logWriter, ai/cache
// debug prints) so their output cannot corrupt the renderer's in-place
// frame yet is never lost.
//
// Content arrives already secret-redacted — clicky's logWriter wraps this
// gate in text.LineFilter(RedactSecrets) — so flushing the raw buffered
// bytes preserves redaction.
type stderrGate struct{}

// gatedStderrBuf holds writes that arrived while the interactive renderer
// owned the TTY, in arrival order, until flushGatedStderr runs.
var gatedStderrBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (stderrGate) Write(p []byte) (int, error) {
	if !IsInteractiveRenderActive() {
		return os.Stderr.Write(p)
	}
	gatedStderrBuf.mu.Lock()
	defer gatedStderrBuf.mu.Unlock()
	gatedStderrBuf.buf.Write(p)
	if !IsInteractiveRenderActive() {
		// The renderer released the TTY between the gate check and taking
		// the lock; the teardown flush may already have run, so flush now
		// rather than stranding the bytes until a future render cycle.
		flushGatedStderrLocked()
	}
	return len(p), nil
}

// flushGatedStderr emits everything buffered during the render window to
// os.Stderr and empties the buffer. Called by finishRenderTeardown right
// after the renderer releases the terminal.
func flushGatedStderr() {
	gatedStderrBuf.mu.Lock()
	defer gatedStderrBuf.mu.Unlock()
	flushGatedStderrLocked()
}

func flushGatedStderrLocked() {
	if gatedStderrBuf.buf.Len() == 0 {
		return
	}
	_, _ = os.Stderr.Write(gatedStderrBuf.buf.Bytes())
	gatedStderrBuf.buf.Reset()
}

// GatedStderr returns a writer that wraps os.Stderr but buffers writes
// while the interactive task renderer owns the TTY, flushing them once it
// lets go. The writer is stateless; each Write rechecks ownership, so a
// writer captured before the renderer started still gates correctly.
func GatedStderr() io.Writer {
	return stderrGate{}
}
