package task

import (
	"bytes"
	"sync"
	"testing"

	"github.com/flanksource/commons/logger"
)

// TestLogSerializingWriter_DelegatesToNext verifies that each Write reaches
// the underlying writer unchanged. Not checking serialization here — that's
// in the concurrent test below.
func TestLogSerializingWriter_DelegatesToNext(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	w := newLogSerializingWriter(&mu, &buf, false)

	if _, err := w.Write([]byte("hello\n")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if got := buf.String(); got != "hello\n" {
		t.Fatalf("delegate write: got %q want %q", got, "hello\n")
	}
}

// TestLogSerializingWriter_NilNextIsNoop verifies a nil next writer doesn't
// crash — we drop the bytes silently (better than panicking in shutdown).
func TestLogSerializingWriter_NilNextIsNoop(t *testing.T) {
	var mu sync.Mutex
	w := newLogSerializingWriter(&mu, nil, false)
	n, err := w.Write([]byte("ignored"))
	if err != nil || n != len("ignored") {
		t.Fatalf("nil-next Write: n=%d err=%v, want 7 nil", n, err)
	}
}

// TestLogSerializer_HoldsMutexDuringWrite verifies that a concurrent acquire
// of the shared mutex blocks for the duration of a log Write. This is the
// property that prevents a log line from landing mid-tick.
func TestLogSerializer_HoldsMutexDuringWrite(t *testing.T) {
	var mu sync.Mutex

	// Delay inside the underlying writer to simulate a slow flush. If the
	// serializer holds the mutex during the Write, the competing TryLock
	// must fail until the Write returns.
	slow := &signalWriter{}
	w := newLogSerializingWriter(&mu, slow, false)

	slow.entered = make(chan struct{})
	slow.release = make(chan struct{})
	go func() {
		_, _ = w.Write([]byte("payload"))
	}()
	<-slow.entered
	if mu.TryLock() {
		mu.Unlock()
		t.Fatalf("mutex was not held while Write was in flight")
	}
	close(slow.release)
}

type signalWriter struct {
	entered chan struct{}
	release chan struct{}
}

func (s *signalWriter) Write(p []byte) (int, error) {
	close(s.entered)
	<-s.release
	return len(p), nil
}

// TestLogSerializingWriter_BufferedHoldsUntilFlush verifies interactive-mode
// buffering: writes accumulate in pending and reach the underlying writer
// only when FlushPending runs (the tick's above-frame emission point).
func TestLogSerializingWriter_BufferedHoldsUntilFlush(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	w := newLogSerializingWriter(&mu, &buf, true)

	if _, err := w.Write([]byte("held\n")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("buffered write must not reach next before FlushPending, got %q", buf.String())
	}

	mu.Lock()
	w.FlushPending()
	mu.Unlock()
	if got := buf.String(); got != "held\n" {
		t.Fatalf("FlushPending: got %q want %q", got, "held\n")
	}

	// Flush is one-shot: a second flush with nothing pending emits nothing.
	mu.Lock()
	w.FlushPending()
	mu.Unlock()
	if got := buf.String(); got != "held\n" {
		t.Fatalf("second FlushPending must be a no-op, got %q", got)
	}
}

// TestLogSerializingWriter_FlushEndsOnFreshLine verifies a pending fragment
// without a trailing newline gets one on flush, so the frame repainted right
// after starts at column 0 instead of merging into the log line.
func TestLogSerializingWriter_FlushEndsOnFreshLine(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	w := newLogSerializingWriter(&mu, &buf, true)

	if _, err := w.Write([]byte("partial")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	mu.Lock()
	w.FlushPending()
	mu.Unlock()
	if got := buf.String(); got != "partial\n" {
		t.Fatalf("FlushPending must terminate the line: got %q want %q", got, "partial\n")
	}
}

// TestInstallUninstall_RoundTripsLoggerOutput verifies the lifecycle hooks
// leave commons' logger in exactly the state they found it.
func TestInstallUninstall_RoundTripsLoggerOutput(t *testing.T) {
	before := logger.GetOutput()
	tm := &Manager{}
	tm.installLogSerializer()
	if logger.GetOutput() == before {
		t.Fatalf("installLogSerializer did not change logger output")
	}
	tm.uninstallLogSerializer()
	if got := logger.GetOutput(); got != before {
		t.Fatalf("uninstallLogSerializer did not restore: got %T want %T", got, before)
	}
}
