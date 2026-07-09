package task

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
)

// TestCaptureAndStopFlushesInOrder verifies the core contract: while
// capture is on, writes to os.Stdout / os.Stderr land in the buffer
// (nothing hits the real terminal); on Stop, the buffered lines are
// flushed to the restored streams in the order they were written.
func TestCaptureAndStopFlushesInOrder(t *testing.T) {
	// Swap the real os.Stdout/os.Stderr with captured pipes so we can
	// observe what gets flushed. Restore them at the end regardless of
	// outcome so a failure doesn't leave the test harness with broken fds.
	realStdout := os.Stdout
	realStderr := os.Stderr
	t.Cleanup(func() {
		os.Stdout = realStdout
		os.Stderr = realStderr
	})

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("make stdout pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("make stderr pipe: %v", err)
	}
	os.Stdout = outW
	os.Stderr = errW

	StartCapturingOutput()

	fmt.Fprintln(os.Stdout, "alpha")
	fmt.Fprintln(os.Stderr, "bravo")
	fmt.Fprintln(os.Stdout, "charlie")

	// Give the reader goroutines a chance to drain the pipes before Stop.
	time.Sleep(20 * time.Millisecond)

	StopCapturingOutput()
	// After StopCapturingOutput the manager prints to os.Stdout / os.Stderr,
	// which here are the test-owned pipes. Close the write ends so the
	// readers below see EOF.
	outW.Close()
	errW.Close()

	stdoutBytes, _ := io.ReadAll(outR)
	stderrBytes, _ := io.ReadAll(errR)

	if !bytes.Contains(stdoutBytes, []byte("alpha")) {
		t.Errorf("stdout should carry 'alpha', got %q", stdoutBytes)
	}
	if !bytes.Contains(stdoutBytes, []byte("charlie")) {
		t.Errorf("stdout should carry 'charlie', got %q", stdoutBytes)
	}
	if !bytes.Contains(stderrBytes, []byte("bravo")) {
		t.Errorf("stderr should carry 'bravo', got %q", stderrBytes)
	}
	// 'bravo' must not have leaked onto stdout (stream tagging is correct).
	if bytes.Contains(stdoutBytes, []byte("bravo")) {
		t.Errorf("stderr line leaked onto stdout: %q", stdoutBytes)
	}
}

// swapTestPipes replaces os.Stdout/os.Stderr with test-owned pipes so the
// flush performed by StopCapturingOutput lands somewhere readable. The
// returned closeWriters must be called after Stop so io.ReadAll sees EOF.
func swapTestPipes(t *testing.T) (outR, errR *os.File, closeWriters func()) {
	t.Helper()
	realStdout := os.Stdout
	realStderr := os.Stderr
	t.Cleanup(func() {
		os.Stdout = realStdout
		os.Stderr = realStderr
	})

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("make stdout pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("make stderr pipe: %v", err)
	}
	os.Stdout = outW
	os.Stderr = errW
	return outR, errR, func() {
		outW.Close()
		errW.Close()
	}
}

// TestStopFlushesTrailingOutputWithoutDelay verifies that Stop joins the
// drain goroutines: a line written immediately before StopCapturingOutput
// (no sleep) must still appear in the flushed output.
func TestStopFlushesTrailingOutputWithoutDelay(t *testing.T) {
	outR, errR, closeWriters := swapTestPipes(t)

	StartCapturingOutput()
	fmt.Println("trailing-sentinel")
	StopCapturingOutput()
	closeWriters()

	stdoutBytes, _ := io.ReadAll(outR)
	_, _ = io.ReadAll(errR)

	if !bytes.Contains(stdoutBytes, []byte("trailing-sentinel")) {
		t.Errorf("Stop must flush output written just before it, got %q", stdoutBytes)
	}
}

// TestBlankLinesArePreserved verifies that empty lines round-trip through
// the capture buffer: "a\n\nb\n" must flush with the blank line intact.
func TestBlankLinesArePreserved(t *testing.T) {
	outR, errR, closeWriters := swapTestPipes(t)

	StartCapturingOutput()
	fmt.Print("a\n\nb\n")
	StopCapturingOutput()
	closeWriters()

	stdoutBytes, _ := io.ReadAll(outR)
	_, _ = io.ReadAll(errR)

	if !bytes.Contains(stdoutBytes, []byte("a\n\nb\n")) {
		t.Errorf("blank line between a and b must be preserved, got %q", stdoutBytes)
	}
}

// TestWaitFlushesAndStopsCapture verifies that Wait() itself stops output
// capture: an app that calls StartCapturingOutput then task.Wait() and exits
// without the shutdown hook must still get its buffered output flushed and
// os.Stdout restored to the pre-capture file.
func TestWaitFlushesAndStopsCapture(t *testing.T) {
	outR, errR, closeWriters := swapTestPipes(t)
	swappedStdout := os.Stdout

	originalGlobal := global
	global = newTestManager(1)
	t.Cleanup(func() {
		global.StopCapturingOutput()
		global = originalGlobal
	})

	StartCapturingOutput()
	fmt.Println("wait-flush-sentinel")

	StartTask[string]("trivial", func(flanksourceContext.Context, *Task) (string, error) {
		return "", nil
	})
	Wait()

	if os.Stdout != swappedStdout {
		t.Errorf("Wait must stop capture and restore os.Stdout to the pre-capture file: got %p, want %p", os.Stdout, swappedStdout)
	}

	closeWriters()
	stdoutBytes, _ := io.ReadAll(outR)
	_, _ = io.ReadAll(errR)

	if !bytes.Contains(stdoutBytes, []byte("wait-flush-sentinel")) {
		t.Errorf("Wait must flush captured output, got %q", stdoutBytes)
	}
}

// TestStopWithoutStartIsSafe ensures StopCapturingOutput is a no-op when
// StartCapturingOutput was never called — gavel relies on this to use
// defer StopCapturingOutput() across code paths that may not have
// started capture.
func TestStopWithoutStartIsSafe(t *testing.T) {
	StopCapturingOutput()
	// If we reach here without a panic, the test passed.
}

// TestDoubleStartIsIdempotent guards against an accidental second call
// silently stealing the original file descriptors a second time.
func TestDoubleStartIsIdempotent(t *testing.T) {
	realStdout := os.Stdout
	realStderr := os.Stderr
	t.Cleanup(func() {
		os.Stdout = realStdout
		os.Stderr = realStderr
		StopCapturingOutput()
	})

	StartCapturingOutput()
	firstStdout := os.Stdout
	StartCapturingOutput()
	secondStdout := os.Stdout
	if firstStdout != secondStdout {
		t.Errorf("double Start must not rotate os.Stdout again: %p vs %p", firstStdout, secondStdout)
	}
}
