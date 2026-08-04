package task

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
)

// withSwappedStderr redirects os.Stderr to a pipe, returns a function that
// closes the writer and yields the captured bytes, plus a cleanup function
// that restores the original os.Stderr. The reader runs in a goroutine so
// writes don't block on the pipe buffer.
func withSwappedStderr(t *testing.T) (collect func() string, cleanup func()) {
	t.Helper()
	original := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w

	var (
		buf  bytes.Buffer
		mu   sync.Mutex
		done = make(chan struct{})
	)
	go func() {
		defer close(done)
		tmp := make([]byte, 1024)
		for {
			n, rerr := r.Read(tmp)
			if n > 0 {
				mu.Lock()
				buf.Write(tmp[:n])
				mu.Unlock()
			}
			if rerr != nil {
				return
			}
		}
	}()

	collect = func() string {
		_ = w.Close()
		<-done
		_ = r.Close()
		mu.Lock()
		defer mu.Unlock()
		return buf.String()
	}
	cleanup = func() {
		os.Stderr = original
	}
	return collect, cleanup
}

// withGlobalRenderState mutates the package-level global manager's render
// flags, returning a restore func. Tests that touch global state must be
// run sequentially — they're already in the same package, so go test
// runs them in order within this file.
func withGlobalRenderState(t *testing.T, interactive, ownsTTY bool) func() {
	t.Helper()
	prevInteractive := global.isInteractive.Load()
	global.mu.Lock()
	prevOwns := global.renderOwnsTTY
	global.renderOwnsTTY = ownsTTY
	global.mu.Unlock()
	global.isInteractive.Store(interactive)
	return func() {
		global.isInteractive.Store(prevInteractive)
		global.mu.Lock()
		global.renderOwnsTTY = prevOwns
		global.mu.Unlock()
	}
}

// resetGatedStderr clears the package-level gate buffer so buffered bytes
// from one test cannot leak into another's flush assertions.
func resetGatedStderr(t *testing.T) {
	t.Helper()
	clear := func() {
		gatedStderrBuf.mu.Lock()
		gatedStderrBuf.buf.Reset()
		gatedStderrBuf.mu.Unlock()
	}
	clear()
	t.Cleanup(clear)
}

func TestStderrGate(t *testing.T) {
	cases := []struct {
		name        string
		interactive bool
		ownsTTY     bool
		input       string
		wantWritten string // what should reach os.Stderr
	}{
		{
			name:        "non-interactive forwards",
			interactive: false,
			ownsTTY:     false,
			input:       "hello\n",
			wantWritten: "hello\n",
		},
		{
			name:        "interactive but renderer not yet owns tty forwards",
			interactive: true,
			ownsTTY:     false,
			input:       "before render\n",
			wantWritten: "before render\n",
		},
		{
			name:        "interactive and renderer owns tty buffers",
			interactive: true,
			ownsTTY:     true,
			input:       "held until flush\n",
			wantWritten: "",
		},
		{
			name:        "owns tty but not interactive forwards",
			interactive: false,
			ownsTTY:     true,
			input:       "plain mode passes\n",
			wantWritten: "plain mode passes\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetGatedStderr(t)
			collect, cleanup := withSwappedStderr(t)
			defer cleanup()
			restore := withGlobalRenderState(t, tc.interactive, tc.ownsTTY)
			defer restore()

			n, err := io.WriteString(GatedStderr(), tc.input)
			if err != nil {
				t.Fatalf("write returned error: %v", err)
			}
			if n != len(tc.input) {
				t.Fatalf("write returned n=%d want %d", n, len(tc.input))
			}

			got := collect()
			if got != tc.wantWritten {
				t.Fatalf("captured stderr = %q, want %q", got, tc.wantWritten)
			}
		})
	}
}

// TestStderrGate_RechecksPerWrite confirms a writer captured before the
// renderer takes over still gates correctly when the renderer later
// acquires the TTY — this is the format.go logWriter scenario, where
// the writer is bound at package init. The write that lands during the
// render window is buffered and appears once the teardown flush runs.
func TestStderrGate_RechecksPerWrite(t *testing.T) {
	resetGatedStderr(t)
	collect, cleanup := withSwappedStderr(t)
	defer cleanup()

	w := GatedStderr()

	restore1 := withGlobalRenderState(t, false, false)
	_, _ = fmt.Fprint(w, "pre\n")
	restore1()

	restore2 := withGlobalRenderState(t, true, true)
	_, _ = fmt.Fprint(w, "during\n")
	restore2()
	flushGatedStderr() // what stopRender's teardown does

	restore3 := withGlobalRenderState(t, false, false)
	_, _ = fmt.Fprint(w, "post\n")
	restore3()

	got := collect()
	want := "pre\nduring\npost\n"
	if got != want {
		t.Fatalf("captured stderr = %q, want %q", got, want)
	}
}

// F4: writes that land while the renderer owns the TTY are buffered, not
// dropped, and flushGatedStderr emits them once the renderer lets go.
func TestStderrGate_BuffersAndFlushesAfterRender(t *testing.T) {
	resetGatedStderr(t)
	collect, cleanup := withSwappedStderr(t)
	defer cleanup()

	restore := withGlobalRenderState(t, true, true)
	_, _ = io.WriteString(GatedStderr(), "first buffered\n")
	_, _ = io.WriteString(GatedStderr(), "second buffered\n")
	restore()

	flushGatedStderr()
	// A second flush with nothing pending must not re-emit.
	flushGatedStderr()

	got := collect()
	want := "first buffered\nsecond buffered\n"
	if got != want {
		t.Fatalf("captured stderr = %q, want %q", got, want)
	}
}

// stopRender's teardown is the production flush point: bytes buffered during
// the render window must reach stderr once the render lifecycle tears down.
func TestStopRender_FlushesGatedStderr(t *testing.T) {
	resetGatedStderr(t)
	collect, cleanup := withSwappedStderr(t)
	defer cleanup()

	restore := withGlobalRenderState(t, true, true)
	_, _ = io.WriteString(GatedStderr(), "flushed by stopRender\n")
	restore()

	tm := newTestManager(1)
	t.Cleanup(func() { close(tm.shutdown) })
	tm.stopRender()

	if got := collect(); !bytes.Contains([]byte(got), []byte("flushed by stopRender\n")) {
		t.Fatalf("stopRender teardown must flush the gated stderr buffer, got %q", got)
	}
}
