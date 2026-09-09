package exec

import (
	"io"
	"sync"
)

// ExecLogger buffers a subprocess's stdout/stderr. The subprocess writes the
// capture storage (via the writers returned by GetStdoutWriter/GetStderrWriter) on the
// os/exec copy goroutines while callers read them via GetStdout/GetStderr —
// so every access is guarded by mu.
type ExecLogger struct {
	mu     sync.Mutex
	stdout captureBuffer
	stderr captureBuffer
	Stderr io.Writer
	Stdout io.Writer
}

func (e *ExecLogger) GetStderr() string {
	if e == nil {
		return ""
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stderr.String()
}

func (e *ExecLogger) GetOutput() string {
	if e == nil {
		return ""
	}
	return e.GetStderr() + e.GetStdout()

}

func (e *ExecLogger) GetStdout() string {
	if e == nil {
		return ""
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stdout.String()
}

func NewExecLogger() *ExecLogger {
	return &ExecLogger{}
}

// StreamTail is a bounded view of one stream: Data, preceded by Offset bytes
// the capture has already discarded. Offset is 0 until the ring rolls over, and
// Offset+len(Data) is the total the stream has carried — so a consumer holding
// an earlier tail can tell whether the new one continues it or skipped past it.
type StreamTail struct {
	Data   string
	Offset int64
}

// End is the absolute stream offset one past the last retained byte.
func (t StreamTail) End() int64 { return t.Offset + int64(len(t.Data)) }

// Tail returns both captured streams with their absolute stream positions.
func (l *ExecLogger) Tail() (stdout, stderr StreamTail) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return StreamTail{Data: l.stdout.String(), Offset: l.stdout.base()},
		StreamTail{Data: l.stderr.String(), Offset: l.stderr.base()}
}

func (l *ExecLogger) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stdout.Reset()
	l.stderr.Reset()
}

// WithTee returns a new ExecLogger that tees logs to stdout/stderr as well.
func (l *ExecLogger) Tee(stdout, stderr io.Writer) *ExecLogger {
	l.Stderr = stderr
	l.Stdout = stdout
	return l
}

// teeAlso adds destinations to the existing tee instead of replacing it, so a
// supervisor can follow the streams without taking the tee away from a caller
// who already asked for one via Stream or Debug.
func (l *ExecLogger) teeAlso(stdout, stderr io.Writer) *ExecLogger {
	l.Stdout = alsoWrite(l.Stdout, stdout)
	l.Stderr = alsoWrite(l.Stderr, stderr)
	return l
}

func alsoWrite(existing, extra io.Writer) io.Writer {
	if extra == nil {
		return existing
	}
	if existing == nil {
		return extra
	}
	return io.MultiWriter(existing, extra)
}

func (l *ExecLogger) GetStdoutWriter() (writer io.Writer) {
	writer = &lockedWriter{mu: &l.mu, buf: &l.stdout}
	if l.Stdout != nil {
		writer = io.MultiWriter(l.Stdout, writer)
	}
	return
}

func (l *ExecLogger) GetStderrWriter() (writer io.Writer) {
	writer = &lockedWriter{mu: &l.mu, buf: &l.stderr}
	if l.Stderr != nil {
		writer = io.MultiWriter(l.Stderr, writer)
	}
	return
}

// lockedWriter serializes buffer writes from the os/exec copy goroutines with
// GetStdout/GetStderr reads under the same ExecLogger mutex.
type lockedWriter struct {
	mu  *sync.Mutex
	buf *captureBuffer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

// setCaptureLimit caps each captured stream independently and retains the
// newest bytes when existing output exceeds the new limit.
func (l *ExecLogger) setCaptureLimit(maxBytes int) {
	if maxBytes <= 0 {
		panic("capture limit must be positive")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stdout.setLimit(maxBytes)
	l.stderr.setLimit(maxBytes)
}

// captureLimit reports the configured stdout cap, 0 when unbounded.
func (l *ExecLogger) captureLimit() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stdout.limit
}

func (l *ExecLogger) clone() *ExecLogger {
	l.mu.Lock()
	defer l.mu.Unlock()
	return &ExecLogger{
		stdout: captureBuffer{limit: l.stdout.limit},
		stderr: captureBuffer{limit: l.stderr.limit},
		Stdout: l.Stdout,
		Stderr: l.Stderr,
	}
}

// captureBuffer is an unbounded byte slice by default and a fixed-size ring
// when limit is positive. Limited writes always report the full input length
// so io.MultiWriter and stdio protocol readers are not affected by truncation.
//
// written counts every byte the buffer has ever accepted, including the ones a
// ring rollover has since discarded. It is what lets a reader say WHERE the
// retained bytes sit in the stream (see base), so a consumer streaming the tail
// can tell an append from a rollover instead of guessing by comparing text.
type captureBuffer struct {
	data    []byte
	start   int
	size    int
	limit   int
	written int64
}

func (b *captureBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if n == 0 {
		return 0, nil
	}
	b.written += int64(n)
	if b.limit == 0 {
		b.data = append(b.data, p...)
		b.size = len(b.data)
		return n, nil
	}
	if len(b.data) != b.limit {
		b.data = make([]byte, b.limit)
	}
	if n >= b.limit {
		copy(b.data, p[n-b.limit:])
		b.start = 0
		b.size = b.limit
		return n, nil
	}

	end := (b.start + b.size) % b.limit
	first := min(n, b.limit-end)
	copy(b.data[end:], p[:first])
	copy(b.data, p[first:])
	total := b.size + n
	if total > b.limit {
		b.start = (b.start + total - b.limit) % b.limit
		b.size = b.limit
	} else {
		b.size = total
	}
	return n, nil
}

// base is the absolute stream offset of the first retained byte — 0 until a
// rollover discards something, and thereafter the count of discarded bytes.
func (b *captureBuffer) base() int64 { return b.written - int64(b.size) }

func (b *captureBuffer) String() string {
	if b.size == 0 {
		return ""
	}
	if b.limit == 0 || b.start+b.size <= len(b.data) {
		return string(b.data[b.start : b.start+b.size])
	}
	output := make([]byte, b.size)
	first := copy(output, b.data[b.start:])
	copy(output[first:], b.data[:b.size-first])
	return string(output)
}

// Reset restarts the stream: a re-Run of the same Process is a new stream, so
// the absolute offsets restart at zero along with the retained bytes.
func (b *captureBuffer) Reset() {
	if b.limit == 0 {
		b.data = b.data[:0]
	}
	b.start = 0
	b.size = 0
	b.written = 0
}

// setLimit re-anchors the ring, keeping the newest maxBytes. written is
// preserved: capping changes what is retained, not what the stream has carried.
func (b *captureBuffer) setLimit(maxBytes int) {
	if maxBytes <= 0 {
		panic("capture limit must be positive")
	}
	current := b.String()
	if len(current) > maxBytes {
		current = current[len(current)-maxBytes:]
	}
	b.data = make([]byte, maxBytes)
	copy(b.data, current)
	b.start = 0
	b.size = len(current)
	b.limit = maxBytes
}
