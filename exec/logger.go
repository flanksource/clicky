package exec

import (
	"bytes"
	"io"
)

type ExecLogger struct {
	stdout *bytes.Buffer
	stderr *bytes.Buffer
	Stderr io.Writer
	Stdout io.Writer
}

func (e *ExecLogger) GetStderr() string {
	if e == nil {
		return ""
	}
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
	writer = l.stdout
	if l.Stdout != nil {
		writer = io.MultiWriter(l.Stdout, writer)
	}
	return
}

func (l *ExecLogger) GetStderrWriter() (writer io.Writer) {
	writer = l.stderr
	if l.Stderr != nil {
		writer = io.MultiWriter(l.Stderr, writer)
	}
	return
}
