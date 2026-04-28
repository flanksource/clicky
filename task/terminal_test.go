//go:build !windows

package task_test

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

type terminalFlags struct {
	iflag uint64
	oflag uint64
	lflag uint64
}

// Critical flags that MakeRaw disables
const criticalLflagMask = unix.ISIG | unix.ICANON | unix.ECHO | unix.IEXTEN
const criticalIflagMask = unix.ICRNL | unix.IXON | unix.BRKINT
const criticalOflagMask = unix.OPOST

func assertFlagsMatch(t *testing.T, before, after terminalFlags) {
	t.Helper()

	if before.lflag&criticalLflagMask != after.lflag&criticalLflagMask {
		t.Errorf("lflag mismatch: before=%#x after=%#x (mask=%#x)",
			before.lflag&criticalLflagMask, after.lflag&criticalLflagMask, uint64(criticalLflagMask))
	}
	if before.iflag&criticalIflagMask != after.iflag&criticalIflagMask {
		t.Errorf("iflag mismatch: before=%#x after=%#x (mask=%#x)",
			before.iflag&criticalIflagMask, after.iflag&criticalIflagMask, uint64(criticalIflagMask))
	}
	if before.oflag&criticalOflagMask != after.oflag&criticalOflagMask {
		t.Errorf("oflag mismatch: before=%#x after=%#x (mask=%#x)",
			before.oflag&criticalOflagMask, after.oflag&criticalOflagMask, uint64(criticalOflagMask))
	}
}

var helperBinary string

func TestMain(m *testing.M) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		os.Exit(0)
	}

	tmpFile, err := os.CreateTemp("", "terminal_test_helper_*")
	if err != nil {
		panic(err)
	}
	helperBinary = tmpFile.Name()
	tmpFile.Close()

	cmd := exec.Command("go", "build", "-o", helperBinary, "./testdata/terminal_test_helper.go")
	cmd.Dir = findTaskDir()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build test helper: " + err.Error())
	}

	code := m.Run()
	os.Remove(helperBinary)
	os.Exit(code)
}

func findTaskDir() string {
	dir, err := os.Getwd()
	if err != nil {
		panic("failed to get working directory: " + err.Error())
	}

	// Walk up the directory tree to find the project root (contains go.mod),
	// then return the "task" subdirectory. This avoids assuming that tests
	// are always invoked from a specific directory.
	d := dir
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return filepath.Join(d, "task")
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}

	// Fallback: if we're already inside the task directory, use it directly.
	return dir
}

func TestTerminalStateNormalExit(t *testing.T) {
	ptmx, before, cmd := startHelper(t, "normal")
	defer ptmx.Close()

	requireExitWithin(t, cmd, ptmx, 15*time.Second)

	after := getTerminalFlags(t, int(ptmx.Fd()))
	assertFlagsMatch(t, before, after)
}

func TestTerminalStateSigint(t *testing.T) {
	ptmx, before, cmd := startHelper(t, "sigint")
	defer ptmx.Close()

	waitForReady(t, ptmx)
	require.NoError(t, cmd.Process.Signal(syscall.SIGINT))

	requireExitWithin(t, cmd, ptmx, 15*time.Second)

	after := getTerminalFlags(t, int(ptmx.Fd()))
	assertFlagsMatch(t, before, after)
}

func TestTerminalStateDoubleSigint(t *testing.T) {
	ptmx, before, cmd := startHelper(t, "sigint_double")
	defer ptmx.Close()

	waitForReady(t, ptmx)

	// Drain pty output in the background before sending signals
	// to prevent the child from blocking on writes during shutdown.
	go func() { _, _ = io.Copy(io.Discard, ptmx) }()

	require.NoError(t, cmd.Process.Signal(syscall.SIGINT))
	time.Sleep(500 * time.Millisecond)
	require.NoError(t, cmd.Process.Signal(syscall.SIGINT))

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for process to exit")
	}

	after := getTerminalFlags(t, int(ptmx.Fd()))
	assertFlagsMatch(t, before, after)
}

func TestTerminalStatePanic(t *testing.T) {
	ptmx, before, cmd := startHelper(t, "panic")
	defer ptmx.Close()

	requireExitWithin(t, cmd, ptmx, 15*time.Second)

	after := getTerminalFlags(t, int(ptmx.Fd()))
	assertFlagsMatch(t, before, after)
}

func startHelper(t *testing.T, mode string) (*os.File, terminalFlags, *exec.Cmd) {
	t.Helper()
	cmd := exec.Command(helperBinary)
	cmd.Env = append(os.Environ(), "EXIT_MODE="+mode)

	ptmx, err := pty.Start(cmd)
	require.NoError(t, err)

	before := getTerminalFlags(t, int(ptmx.Fd()))
	return ptmx, before, cmd
}

func requireExitWithin(t *testing.T, cmd *exec.Cmd, ptmx *os.File, timeout time.Duration) {
	t.Helper()

	// Drain pty output in the background to prevent the child process
	// from blocking on writes to stderr during shutdown.
	go func() { _, _ = io.Copy(io.Discard, ptmx) }()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-done:
	case <-time.After(timeout):
		cmd.Process.Kill()
		t.Fatal("timeout waiting for process to exit")
	}
}

func waitForReady(t *testing.T, ptmx *os.File) {
	t.Helper()
	buf := make([]byte, 4096)
	deadline := time.Now().Add(15 * time.Second)
	accumulated := ""

	for time.Now().Before(deadline) {
		ptmx.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, err := ptmx.Read(buf)
		if n > 0 {
			accumulated += string(buf[:n])
			if strings.Contains(accumulated, "READY") {
				return
			}
		}
		if err != nil {
			continue
		}
	}
	t.Fatalf("timeout waiting for READY signal, got: %q", accumulated)
}
