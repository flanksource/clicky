package task

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// OutputEntry represents a captured stdout/stderr line with metadata
type OutputEntry struct {
	Timestamp time.Time
	Stream    string // "stdout" or "stderr"
	Line      string
}

// bufferingWriter captures writes to a buffer with timestamps.
// It accumulates partial lines between Write calls so that a logical
// line split across multiple reads is emitted as a single OutputEntry.
type bufferingWriter struct {
	stream    string
	manager   *Manager
	remainder string // partial line carried over from the previous Write
}

func (w *bufferingWriter) Write(p []byte) (n int, err error) {
	w.manager.bufferMutex.Lock()
	defer w.manager.bufferMutex.Unlock()

	data := w.remainder + string(p)
	w.remainder = ""

	// Split by newlines and add each complete line to buffer
	lines := strings.Split(data, "\n")
	for i, line := range lines {
		if i == len(lines)-1 {
			// Last element: if the input didn't end with '\n' this is an
			// incomplete line — keep it as the remainder for the next Write.
			w.remainder = line
			break
		}
		if line == "" {
			continue
		}
		w.manager.outputBuffer = append(w.manager.outputBuffer, OutputEntry{
			Timestamp: time.Now(),
			Stream:    w.stream,
			Line:      line,
		})
	}

	return len(p), nil
}

// Flush emits any remaining partial line that was not terminated by a newline.
func (w *bufferingWriter) Flush() {
	w.manager.bufferMutex.Lock()
	defer w.manager.bufferMutex.Unlock()

	if w.remainder != "" {
		w.manager.outputBuffer = append(w.manager.outputBuffer, OutputEntry{
			Timestamp: time.Now(),
			Stream:    w.stream,
			Line:      w.remainder,
		})
		w.remainder = ""
	}
}

// StartCapturingOutput redirects stdout/stderr to internal buffer
func (tm *Manager) StartCapturingOutput() {
	tm.bufferMutex.Lock()
	defer tm.bufferMutex.Unlock()

	if tm.capturingOutput {
		return
	}

	tm.originalStdout = os.Stdout
	tm.originalStderr = os.Stderr
	tm.outputBuffer = []OutputEntry{}

	// Create pipes for stdout and stderr
	tm.stdoutReader, tm.stdoutWriter, _ = os.Pipe()
	tm.stderrReader, tm.stderrWriter, _ = os.Pipe()

	os.Stdout = tm.stdoutWriter
	os.Stderr = tm.stderrWriter

	// Start goroutines to read from pipes and buffer output.
	// Each goroutine owns a single bufferingWriter so that partial lines
	// are correctly accumulated across successive reads.
	go func() {
		w := &bufferingWriter{stream: "stdout", manager: tm}
		buf := make([]byte, 4096)
		for {
			n, err := tm.stdoutReader.Read(buf)
			if n > 0 {
				_, _ = w.Write(buf[:n])
			}
			if err != nil {
				w.Flush()
				return
			}
		}
	}()

	go func() {
		w := &bufferingWriter{stream: "stderr", manager: tm}
		buf := make([]byte, 4096)
		for {
			n, err := tm.stderrReader.Read(buf)
			if n > 0 {
				_, _ = w.Write(buf[:n])
			}
			if err != nil {
				w.Flush()
				return
			}
		}
	}()

	tm.capturingOutput = true
}

// StopCapturingOutput restores stdout/stderr and prints buffered output
func (tm *Manager) StopCapturingOutput() {
	tm.bufferMutex.Lock()
	capturing := tm.capturingOutput
	tm.capturingOutput = false
	tm.bufferMutex.Unlock()

	if !capturing {
		return
	}

	// Close writers first to signal reader goroutines to exit
	if tm.stdoutWriter != nil {
		_ = tm.stdoutWriter.Close()
	}
	if tm.stderrWriter != nil {
		_ = tm.stderrWriter.Close()
	}

	// Restore original stdout/stderr
	os.Stdout = tm.originalStdout
	os.Stderr = tm.originalStderr

	// Close readers after restoring original fds
	if tm.stdoutReader != nil {
		_ = tm.stdoutReader.Close()
	}
	if tm.stderrReader != nil {
		_ = tm.stderrReader.Close()
	}

	// Print all buffered output
	tm.bufferMutex.Lock()
	buffer := make([]OutputEntry, len(tm.outputBuffer))
	copy(buffer, tm.outputBuffer)
	tm.bufferMutex.Unlock()

	for _, entry := range buffer {
		if entry.Stream == "stdout" {
			_, _ = fmt.Fprintln(os.Stdout, entry.Line)
		} else {
			_, _ = fmt.Fprintln(os.Stderr, entry.Line)
		}
	}
}
