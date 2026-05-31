package exec

import (
	"bytes"
	"io"
	"sync"
)

// ExecLogger buffers a subprocess's stdout/stderr. The subprocess writes the
// buffers (via the writers returned by GetStdoutWriter/GetStderrWriter) on the
// os/exec copy goroutines while callers read them via GetStdout/GetStderr —
// so every buffer access is guarded by mu. bytes.Buffer is not safe for
// concurrent read/write on its own.
type ExecLogger struct {
	mu     sync.Mutex
	stdout *bytes.Buffer
	stderr *bytes.Buffer
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
	e := &ExecLogger{
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}

	return e
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
	writer = &lockedWriter{mu: &l.mu, buf: l.stdout}
	if l.Stdout != nil {
		writer = io.MultiWriter(l.Stdout, writer)
	}
	return
}

func (l *ExecLogger) GetStderrWriter() (writer io.Writer) {
	writer = &lockedWriter{mu: &l.mu, buf: l.stderr}
	if l.Stderr != nil {
		writer = io.MultiWriter(l.Stderr, writer)
	}
	return
}

// lockedWriter serializes buffer writes from the os/exec copy goroutines with
// GetStdout/GetStderr reads under the same ExecLogger mutex.
type lockedWriter struct {
	mu  *sync.Mutex
	buf *bytes.Buffer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}
