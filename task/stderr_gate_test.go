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
			name:        "interactive and renderer owns tty drops",
			interactive: true,
			ownsTTY:     true,
			input:       "should be dropped\n",
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
// the writer is bound at package init.
func TestStderrGate_RechecksPerWrite(t *testing.T) {
	collect, cleanup := withSwappedStderr(t)
	defer cleanup()

	w := GatedStderr()

	restore1 := withGlobalRenderState(t, false, false)
	_, _ = fmt.Fprint(w, "pre\n")
	restore1()

	restore2 := withGlobalRenderState(t, true, true)
	_, _ = fmt.Fprint(w, "during\n")
	restore2()

	restore3 := withGlobalRenderState(t, false, false)
	_, _ = fmt.Fprint(w, "post\n")
	restore3()

	got := collect()
	want := "pre\npost\n"
	if got != want {
		t.Fatalf("captured stderr = %q, want %q", got, want)
	}
}
