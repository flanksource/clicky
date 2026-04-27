package clicky

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/flanksource/clicky/api"
	clickytext "github.com/flanksource/clicky/text"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

type terminalFlags struct {
	iflag uint64
	oflag uint64
	lflag uint64
}

const criticalLflagMask = unix.ISIG | unix.ICANON | unix.ECHO | unix.IEXTEN
const criticalIflagMask = unix.ICRNL | unix.IXON | unix.BRKINT
const criticalOflagMask = unix.OPOST

var promptHelperBinary string

func TestMain(m *testing.M) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		tmpFile, err := os.CreateTemp("", "prompt_test_helper_*")
		if err != nil {
			panic(err)
		}
		promptHelperBinary = tmpFile.Name()
		_ = tmpFile.Close()

		cmd := exec.Command("go", "build", "-o", promptHelperBinary, "./testdata/prompt_test_helper.go")
		cmd.Dir = findProjectRoot()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			panic("failed to build prompt test helper: " + err.Error())
		}
	}

	code := m.Run()
	if promptHelperBinary != "" {
		_ = os.Remove(promptHelperBinary)
	}
	os.Exit(code)
}

func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic("failed to get working directory: " + err.Error())
	}

	current := dir
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return dir
}

type capturedPTY struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *capturedPTY) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}

func (c *capturedPTY) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

type promptPrettyItem struct {
	label string
}

func (p promptPrettyItem) Pretty() api.Text {
	return api.Text{Content: p.label}
}

type promptStringerItem struct {
	label string
}

func (p promptStringerItem) String() string {
	return p.label
}

func TestDefaultPromptLabelUsesPrettyFirst(t *testing.T) {
	got := defaultPromptLabel(promptPrettyItem{label: "pretty-label"}, nil)
	if got != "pretty-label" {
		t.Fatalf("expected pretty label, got %q", got)
	}
}

func TestDefaultPromptLabelFallsBackToFmt(t *testing.T) {
	got := defaultPromptLabel(promptStringerItem{label: "stringer-label"}, nil)
	if got != "stringer-label" {
		t.Fatalf("expected stringer label, got %q", got)
	}
}

func TestClampPromptPageSizeUsesTerminalHeightWhenUnset(t *testing.T) {
	got := clampPromptPageSize(0, 50, 18, 1)
	if got != 16 {
		t.Fatalf("expected page size 16, got %d", got)
	}
}

func TestClampPromptPageSizeCapsRequestedSizeToTerminalHeight(t *testing.T) {
	got := clampPromptPageSize(40, 50, 18, 1)
	if got != 16 {
		t.Fatalf("expected page size 16, got %d", got)
	}
}

func TestClampPromptPageSizeKeepsRequestedSizeWithinAvailableHeight(t *testing.T) {
	got := clampPromptPageSize(6, 50, 18, 1)
	if got != 6 {
		t.Fatalf("expected page size 6, got %d", got)
	}
}

func TestClampPromptPageSizeUsesMinimumSelectHeight(t *testing.T) {
	got := clampPromptPageSize(1, 50, 18, 1)
	if got != 5 {
		t.Fatalf("expected page size 5, got %d", got)
	}
}

func TestPromptSelectHeightIncludesTitleAndPaginator(t *testing.T) {
	got := promptSelectHeight(5, 8, 1)
	if got != 7 {
		t.Fatalf("expected prompt height 7, got %d", got)
	}
}

func TestPromptRenderedLineCount(t *testing.T) {
	got := promptRenderedLineCount("Choose the next rollout action", 80)
	if got != 1 {
		t.Fatalf("expected rendered line count 1, got %d", got)
	}
}

func TestPromptAvailableLinesUsesRemainingRows(t *testing.T) {
	got := promptAvailableLines(24, 10)
	if got != 14 {
		t.Fatalf("expected 14 available lines, got %d", got)
	}
}

func TestShouldClearPromptScreenWhenPageSizeExceedsRemainingRows(t *testing.T) {
	if !shouldClearPromptScreen(8, 24, 18) {
		t.Fatalf("expected prompt to clear when page size exceeds remaining rows")
	}
}

func TestShouldNotClearPromptScreenWhenPageFitsRemainingRows(t *testing.T) {
	if shouldClearPromptScreen(6, 24, 18) {
		t.Fatalf("expected prompt to fit without clearing")
	}
}

func TestPrepareSelectPromptClearsScreenAfterTaskTakeover(t *testing.T) {
	output, err := os.CreateTemp("", "prompt_output_*")
	require.NoError(t, err)
	defer os.Remove(output.Name())
	defer output.Close()

	session := &promptSession{
		terminal:       &promptTerminal{output: output},
		tookOverRender: true,
	}

	session.prepareSelectPrompt(8)

	_, err = output.Seek(0, 0)
	require.NoError(t, err)

	data, err := io.ReadAll(output)
	require.NoError(t, err)
	require.Equal(t, promptClearScreenSequence, string(data))
}

func TestPromptChooseConfirmsSelectionAndRestoresTerminal(t *testing.T) {
	skipPromptPTYTests(t)

	ptmx, capture, before, cmd := startPromptHelper(t, "choose_confirm")
	defer ptmx.Close()

	time.Sleep(500 * time.Millisecond)
	_, err := ptmx.Write([]byte("\x1b[B"))
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	_, err = ptmx.Write([]byte{'\r'})
	require.NoError(t, err)

	requireExitWithin(t, cmd, 10*time.Second)
	after := getTerminalFlags(t, int(ptmx.Fd()))
	assertFlagsMatch(t, before, after)

	output := clickytext.StripANSI(capture.String())
	require.Contains(t, output, "RESULT=beta OK=true")
}

func TestPromptChooseCancel(t *testing.T) {
	skipPromptPTYTests(t)

	ptmx, capture, _, cmd := startPromptHelper(t, "choose_cancel")
	defer ptmx.Close()

	time.Sleep(500 * time.Millisecond)
	_, err := ptmx.Write([]byte{27})
	require.NoError(t, err)

	requireExitWithin(t, cmd, 10*time.Second)
	output := clickytext.StripANSI(capture.String())
	require.Contains(t, output, "RESULT= OK=false")
}

func TestPromptMultiSelectConfirmsSelections(t *testing.T) {
	skipPromptPTYTests(t)

	ptmx, capture, _, cmd := startPromptHelper(t, "multi_confirm")
	defer ptmx.Close()

	time.Sleep(500 * time.Millisecond)
	_, err := ptmx.Write([]byte("x"))
	require.NoError(t, err)
	_, err = ptmx.Write([]byte("\x1b[B"))
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	_, err = ptmx.Write([]byte("x"))
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	_, err = ptmx.Write([]byte{'\r'})
	require.NoError(t, err)

	requireExitWithin(t, cmd, 10*time.Second)
	output := clickytext.StripANSI(capture.String())
	require.Contains(t, output, "RESULT=alpha,beta OK=true")
}

func TestPromptTextValidation(t *testing.T) {
	skipPromptPTYTests(t)

	ptmx, capture, _, cmd := startPromptHelper(t, "text_validate")
	defer ptmx.Close()

	waitForCapturedText(t, capture, "> ")
	_, err := ptmx.Write([]byte{'\r'})
	require.NoError(t, err)
	waitForCapturedText(t, capture, "value required")

	_, err = ptmx.Write([]byte("moshe\r"))
	require.NoError(t, err)

	requireExitWithin(t, cmd, 10*time.Second)
	output := clickytext.StripANSI(capture.String())
	require.Contains(t, output, "RESULT=moshe OK=true")
}

func TestPromptTextSecretDoesNotEchoInput(t *testing.T) {
	skipPromptPTYTests(t)

	ptmx, capture, _, cmd := startPromptHelper(t, "secret")
	defer ptmx.Close()

	waitForCapturedText(t, capture, "> ")
	_, err := ptmx.Write([]byte("shh\r"))
	require.NoError(t, err)

	requireExitWithin(t, cmd, 10*time.Second)
	output := clickytext.StripANSI(capture.String())
	require.Contains(t, output, "RESULT_LEN=3 OK=true")
	if strings.Contains(output, "shh") {
		t.Fatalf("secret input was echoed in prompt output: %q", output)
	}
}

func TestPromptStopsTaskRenderingBeforePrompt(t *testing.T) {
	skipPromptPTYTests(t)

	ptmx, capture, _, cmd := startPromptHelper(t, "task_takeover")
	defer ptmx.Close()

	time.Sleep(800 * time.Millisecond)
	_, err := ptmx.Write([]byte("\x1b[B"))
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	_, err = ptmx.Write([]byte{'\r'})
	require.NoError(t, err)

	requireExitWithin(t, cmd, 10*time.Second)
	output := clickytext.StripANSI(capture.String())

	if !strings.Contains(output, "TAKEOVER-MARKER") {
		t.Fatalf("expected task output before prompt takeover, got %q", output)
	}
	require.Contains(t, output, "stop-marker")
	require.Contains(t, output, "OK=true")
}

func skipPromptPTYTests(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("PTY prompt tests only run on darwin and linux")
	}
	if promptHelperBinary == "" {
		t.Skip("prompt helper not built")
	}
}

func startPromptHelper(t *testing.T, mode string) (*os.File, *capturedPTY, terminalFlags, *exec.Cmd) {
	t.Helper()

	cmd := exec.Command(promptHelperBinary)
	cmd.Env = append(os.Environ(),
		"PROMPT_MODE="+mode,
		"TERM=screen-256color",
	)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	require.NoError(t, err)

	capture := &capturedPTY{}
	go func() {
		buffer := make([]byte, 4096)
		pending := ""

		for {
			n, err := ptmx.Read(buffer)
			if n > 0 {
				chunk := buffer[:n]
				_, _ = capture.Write(chunk)
				pending += string(chunk)
				pending = replyToTerminalQueries(t, ptmx, pending)
			}
			if err != nil {
				return
			}
		}
	}()

	before := getTerminalFlags(t, int(ptmx.Fd()))
	return ptmx, capture, before, cmd
}

func replyToTerminalQueries(t *testing.T, ptmx *os.File, pending string) string {
	t.Helper()

	replies := map[string]string{
		"\x1b[6n":         "\x1b[1;1R",
		"\x1b]11;?\x1b\\": "\x1b]11;rgb:0000/0000/0000\x1b\\",
	}

	for query, reply := range replies {
		for {
			index := strings.Index(pending, query)
			if index == -1 {
				break
			}
			_, err := ptmx.Write([]byte(reply))
			require.NoError(t, err)
			pending = pending[:index] + pending[index+len(query):]
		}
	}

	if len(pending) > 128 {
		pending = pending[len(pending)-128:]
	}

	return pending
}

func waitForCapturedText(t *testing.T, capture *capturedPTY, want string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(clickytext.StripANSI(capture.String()), want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("timeout waiting for %q in output: %q", want, clickytext.StripANSI(capture.String()))
}

func requireExitWithin(t *testing.T, cmd *exec.Cmd, timeout time.Duration) {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err == nil {
			return
		}
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			t.Fatalf("helper exited with error: %v", err)
		}
	case <-time.After(timeout):
		_ = cmd.Process.Signal(syscall.SIGKILL)
		t.Fatal("timeout waiting for prompt helper to exit")
	}
}

func assertFlagsMatch(t *testing.T, before, after terminalFlags) {
	t.Helper()

	if before.lflag&criticalLflagMask != after.lflag&criticalLflagMask {
		t.Fatalf("lflag mismatch: before=%#x after=%#x", before.lflag&criticalLflagMask, after.lflag&criticalLflagMask)
	}
	if before.iflag&criticalIflagMask != after.iflag&criticalIflagMask {
		t.Fatalf("iflag mismatch: before=%#x after=%#x", before.iflag&criticalIflagMask, after.iflag&criticalIflagMask)
	}
	if before.oflag&criticalOflagMask != after.oflag&criticalOflagMask {
		t.Fatalf("oflag mismatch: before=%#x after=%#x", before.oflag&criticalOflagMask, after.oflag&criticalOflagMask)
	}
}
