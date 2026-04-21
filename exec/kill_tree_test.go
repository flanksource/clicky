//go:build !windows

package exec

import (
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKillTreeAtomicReapsGrandchildren proves that WithProcessGroup +
// KillTree tears down a subprocess whose grandchildren trap SIGINT/SIGTERM
// — the exact shape that was failing for gavel's testrunner/ui package.
func TestKillTreeAtomicReapsGrandchildren(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only scenario")
	}

	script := `trap '' INT TERM; (trap '' INT TERM; sleep 60) & echo $! > /tmp/clicky_kt_child.pid; wait`
	p := NewExec("sh", "-c", script).WithProcessGroup()

	runDone := make(chan struct{})
	go func() {
		_ = p.Run().Result()
		close(runDone)
	}()

	require.NoError(t, waitFor(func() bool { return p.Pid() > 0 }, 2*time.Second),
		"parent pid should become visible")
	parentPID := p.Pid()

	// Wait for the child to write its pid file.
	var childPID int
	require.NoError(t, waitFor(func() bool {
		data, err := os.ReadFile("/tmp/clicky_kt_child.pid")
		if err != nil {
			return false
		}
		_, err = fmtScanInt(string(data), &childPID)
		return err == nil && childPID > 0
	}, 3*time.Second), "grandchild pid file should appear")
	defer os.Remove("/tmp/clicky_kt_child.pid")

	require.NoError(t, p.KillTree(), "KillTree must succeed")

	// Atomic pgid kill — both pids should be reaped inside WaitDelay (2s).
	require.NoError(t, waitFor(func() bool { return !pidAlive(parentPID) }, 2*time.Second),
		"parent %d must die after pgid SIGKILL", parentPID)
	require.NoError(t, waitFor(func() bool { return !pidAlive(childPID) }, 2*time.Second),
		"grandchild %d must die after pgid SIGKILL", childPID)

	// And — the core fix — Run() must return so the caller unblocks.
	select {
	case <-runDone:
	case <-time.After(4 * time.Second):
		t.Fatal("Process.Run() did not return after KillTree")
	}
}

// TestKillTreeWithoutProcessGroupFallsBack exercises the gopsutil fallback.
// Still racy by design — no WithProcessGroup — so we give it a wider window.
func TestKillTreeWithoutProcessGroupFallsBack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only scenario")
	}

	p := NewExec("sh", "-c", "sleep 60")

	runDone := make(chan struct{})
	go func() {
		_ = p.Run().Result()
		close(runDone)
	}()

	require.NoError(t, waitFor(func() bool { return p.Pid() > 0 }, 2*time.Second))
	pid := p.Pid()

	require.NoError(t, p.KillTree())
	require.NoError(t, waitFor(func() bool { return !pidAlive(pid) }, 3*time.Second),
		"pid %d must be reaped by fallback walk", pid)

	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Process.Run() did not return after KillTree")
	}
}

func TestKillTreeBeforeStartIsNoop(t *testing.T) {
	p := NewExec("sleep", "60").WithProcessGroup()
	assert.NoError(t, p.KillTree())
	assert.Equal(t, 0, p.Pid())
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func waitFor(cond func() bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return &waitTimeoutError{after: timeout}
}

type waitTimeoutError struct{ after time.Duration }

func (e *waitTimeoutError) Error() string { return "condition not met within " + e.after.String() }

// fmtScanInt is a minimal parse helper so the test doesn't pull strconv+strings
// for one call site. Returns the number of bytes consumed or an error.
func fmtScanInt(s string, out *int) (int, error) {
	n := 0
	consumed := 0
	started := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			started = true
			n = n*10 + int(c-'0')
			consumed++
			continue
		}
		if started {
			break
		}
	}
	if !started {
		return 0, &waitTimeoutError{after: 0}
	}
	*out = n
	return consumed, nil
}
