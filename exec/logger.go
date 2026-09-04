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
type captureBuffer struct {
	data  []byte
	start int
	size  int
	limit int
}

func (b *captureBuffer) Write(p []byte) (int, error) {
	written := len(p)
	if written == 0 {
		return 0, nil
	}
	if b.limit == 0 {
		b.data = append(b.data, p...)
		b.size = len(b.data)
		return written, nil
	}
	if len(b.data) != b.limit {
		b.data = make([]byte, b.limit)
	}
	if written >= b.limit {
		copy(b.data, p[written-b.limit:])
		b.start = 0
		b.size = b.limit
		return written, nil
	}

	end := (b.start + b.size) % b.limit
	first := min(written, b.limit-end)
	copy(b.data[end:], p[:first])
	copy(b.data, p[first:])
	total := b.size + written
	if total > b.limit {
		b.start = (b.start + total - b.limit) % b.limit
		b.size = b.limit
	} else {
		b.size = total
	}
	return written, nil
}

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

func (b *captureBuffer) Reset() {
	if b.limit == 0 {
		b.data = b.data[:0]
	}
	b.start = 0
	b.size = 0
}

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
