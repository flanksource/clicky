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
	var err error
	tm.stdoutReader, tm.stdoutWriter, err = os.Pipe()
	if err != nil {
		panic(fmt.Sprintf("task: failed to create capture pipe: %v", err))
	}
	tm.stderrReader, tm.stderrWriter, err = os.Pipe()
	if err != nil {
		panic(fmt.Sprintf("task: failed to create capture pipe: %v", err))
	}
	tm.stdoutDone = make(chan struct{})
	tm.stderrDone = make(chan struct{})

	os.Stdout = tm.stdoutWriter
	os.Stderr = tm.stderrWriter

	// Start goroutines to read from pipes and buffer output.
	// Each goroutine owns a single bufferingWriter so that partial lines
	// are correctly accumulated across successive reads.
	go tm.drainPipe(tm.stdoutReader, "stdout", tm.stdoutDone)
	go tm.drainPipe(tm.stderrReader, "stderr", tm.stderrDone)

	tm.capturingOutput = true
}

// drainPipe reads a capture pipe until EOF, buffering complete lines,
// then flushes the trailing partial line and closes done so
// StopCapturingOutput can join it before snapshotting the buffer.
func (tm *Manager) drainPipe(r *os.File, stream string, done chan struct{}) {
	defer close(done)
	w := &bufferingWriter{stream: stream, manager: tm}
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
		}
		if err != nil {
			w.Flush()
			return
		}
	}
}

// StartCapturingOutput replaces os.Stdout and os.Stderr on the global
// task manager with pipes that buffer everything written until
// StopCapturingOutput is called. The live task renderer is unaffected
// because it captured the original file descriptors at manager init
// time (see Manager.renderer in manager.go). Loggers that captured
// os.Stderr before this call will also keep writing to the real
// terminal — only bare fmt.Print / os.Stderr writes after this call
// get buffered.
func StartCapturingOutput() {
	if global == nil {
		return
	}
	global.StartCapturingOutput()
}

// StopCapturingOutput restores the real os.Stdout and os.Stderr on the
// global task manager and flushes every buffered line to the restored
// streams in the order it was written, tagged by stream of origin.
// Safe to call when capture wasn't started (no-op).
func StopCapturingOutput() {
	if global == nil {
		return
	}
	global.StopCapturingOutput()
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

	// Join the drain goroutines so every buffered byte (including the
	// final partial line) lands in outputBuffer before the snapshot.
	// bufferMutex must not be held here: the drainers' final Write/Flush
	// acquire it.
	<-tm.stdoutDone
	<-tm.stderrDone

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
