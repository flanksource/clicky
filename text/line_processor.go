package text

import (
	"bytes"
	"io"
	"sync"
)

// LineProcessor processes a single line and returns:
//   - out: the processed line (may be modified or original)
//   - skip: whether to skip writing this line
//
// If a processor returns skip=true, the pipeline stops immediately
// and the line is not written. Processors should not panic, but the
// framework will recover if they do and skip the line.
type LineProcessor func(line string) (out string, skip bool)

type lineFilterWriter struct {
	writer   io.Writer
	pipeline []LineProcessor
	buffer   *bytes.Buffer
	mu       sync.Mutex
	bufferMu sync.Mutex
	bufPool  *sync.Pool
}

// WriteString implements io.StringWriter.
func (w *lineFilterWriter) WriteString(s string) (n int, err error) {
	return w.Write([]byte(s))
}

// LineFilter wraps an io.Writer to apply a pipeline of line processors.
// Each line (delimited by \n) is processed through the pipeline in left-to-right order.
// If any processor returns skip=true, the line is discarded.
//
// The returned writer is thread-safe for concurrent writes.
//
// Example:
//
//	redactor := clicky.RedactSecrets()
//	writer := clicky.LineFilter(os.Stdout, redactor)
//	writer.Write([]byte("password=secret\n")) // Writes redacted output
func LineFilter(writer io.Writer, pipeline ...LineProcessor) io.Writer {
	bufPool := &sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}

	return &lineFilterWriter{
		writer:   writer,
		pipeline: pipeline,
		buffer:   new(bytes.Buffer),
		bufPool:  bufPool,
	}
}

func (w *lineFilterWriter) Write(p []byte) (n int, err error) {
	w.bufferMu.Lock()
	defer w.bufferMu.Unlock()

	// Append to buffer
	w.buffer.Write(p)

	// Process complete lines
	for {
		line, err := w.buffer.ReadBytes('\n')
		if err != nil {
			// No complete line, put back partial data
			if len(line) > 0 {
				w.buffer.Write(line)
			}
			break
		}

		// Remove trailing newline for processing
		lineStr := string(line[:len(line)-1])

		// Process through pipeline
		processed, skip := w.processLine(lineStr)
		if skip {
			continue
		}

		// Write to underlying writer with newline
		w.mu.Lock()
		_, writeErr := w.writer.Write([]byte(processed + "\n"))
		w.mu.Unlock()

		if writeErr != nil {
			return len(p), writeErr
		}
	}

	return len(p), nil
}

func (w *lineFilterWriter) processLine(line string) (string, bool) {
	current := line
	for _, processor := range w.pipeline {
		var skip bool

		// Recover from panics
		func() {
			defer func() {
				if r := recover(); r != nil {
					skip = true
				}
			}()
			current, skip = processor(current)
		}()

		if skip {
			return current, true
		}
	}
	return current, false
}
