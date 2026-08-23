package exec

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/icons"
	"github.com/flanksource/clicky/task"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/properties"
	"github.com/samber/lo"
)

var log = logger.GetLogger("exec")

// NewExecf creates a new Process with formatted command string
func NewExecf(cmd string, args ...any) *Process {
	return &Process{
		Cmd:           fmt.Sprintf(cmd, args...),
		log:           log,
		captureOutput: NewExecLogger(),
		Env:           map[string]string{},
		done:          make(chan struct{}),
	}
}

// NewExec creates a new Process with the specified command and arguments
func NewExec(cmd string, args ...string) *Process {
	return &Process{
		Cmd:           cmd,
		log:           log,
		Args:          args,
		captureOutput: NewExecLogger(),
		Env:           map[string]string{},
		done:          make(chan struct{}),
	}
}

// ContainsShellOperators checks if a command contains shell-specific operators
// that require wrapping in a shell (bash -c or sh -c)
func ContainsShellOperators(cmd string) bool {
	shellOps := []string{"|", ">", "<", "2>", "&&", "||", ";", "`", "$("}
	for _, op := range shellOps {
		if strings.Contains(cmd, op) {
			return true
		}
	}
	return false
}

type Process struct {
	log logger.Logger
	// For streaming and capturing output
	captureOutput *ExecLogger
	// For verbose / error logging

	// mu guards the fields written by Run() (cmd, exitCode, Err, Started,
	// running) so that supervisor goroutines can safely call IsRunning,
	// Pid, KillTree, Result, etc. concurrently with an in-flight Run(). The
	// lock is only ever held for brief field reads/writes — never across the
	// blocking cmd.Run()/cmd.Wait() calls — so probes never block on the
	// subprocess lifetime.
	mu sync.RWMutex
	// running reflects whether the subprocess is currently executing. Read by
	// IsRunning instead of peeking cmd.ProcessState, which os/exec only
	// publishes safely after Wait() returns.
	running bool
	// pid is clicky's own copy of cmd.Process.Pid, captured under mu right
	// after cmd.Start(). Probes (Pid/KillTree) read this instead of
	// cmd.Process, whose fields os/exec mutates without a lock we control.
	pid int
	// completed / succeeded are clicky-owned copies of cmd.ProcessState's
	// terminal status, captured under mu once Wait() returns. Probes
	// (Result/IsOK/Status) read these instead of cmd.ProcessState, which
	// os/exec writes during Wait() without a lock we control. completed is
	// true whenever the process finished (normal exit OR signal); succeeded is
	// true only for a clean zero-exit.
	completed bool
	succeeded bool

	Started *time.Time
	cmd     *exec.Cmd
	task    *task.Task
	// ctx, when set (see WithContext), cancels an in-flight Run(): an
	// already-expired context prevents the start, and ctx.Done() firing during
	// execution interrupts the child (escalating to SIGKILL) while Run itself
	// reaps it.
	ctx     context.Context
	Timeout time.Duration
	Env     map[string]string
	// exactEnvironment makes Env the complete child environment instead of
	// merging it over the host environment.
	exactEnvironment bool
	Cwd              string
	Err              error
	Shell            string
	Cmd              string
	Args             []string
	// Consider a non-zero exit code as an error
	SucceedOnNonZero bool
	exitCode         *int
	done             chan struct{} // Closed when Run() completes
	// newProcessGroup, when true, spawns the subprocess in its own POSIX
	// process group (Setpgid) / Windows process group (CREATE_NEW_PROCESS_GROUP)
	// so the whole descendant tree can be terminated atomically via KillTree.
	newProcessGroup bool

	// wantStdioPipe, when true, connects the child's stdin to a writable pipe
	// (Stdin) and its stdout to a readable pipe (StdoutReader) for bidirectional
	// line-protocol traffic (e.g. JSON-RPC). The child's stdout BYPASSES
	// captureOutput so a long-lived server's output does not accumulate
	// unboundedly in memory; stderr stays captured (bounded). stdin/stdoutR are
	// the parent-side ends, published under mu once Run() has started the child.
	wantStdioPipe bool
	input         io.Reader
	stdin         io.WriteCloser
	stdoutR       io.Reader
}

// ExecResult contains the result of a command execution with structured output and metadata.
type ExecResult struct {
	Stdout   string        `json:"stdout,omitempty"`
	Stderr   string        `json:"stderr,omitempty"`
	Status   string        `json:"status,omitempty"`
	ExitCode int           `json:"exit_code,omitempty"`
	Started  *time.Time    `json:"started,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`
	PID      int           `json:"pid,omitempty"`
	Command  string        `json:"command,omitempty"`
	Args     []string      `json:"args,omitempty"`
	Error    error         `json:"error,omitempty"`
	process  *Process      `json:"-"`
}

func (r ExecResult) IsOk() bool {
	return r.Error == nil && r.ExitCode == 0
}

func (r ExecResult) IsPending() bool {
	return r.Status == "pending"
}

func (r ExecResult) IsCompleted() bool {
	return r.ExitCode >= 0
}

func (r ExecResult) Output() string {
	return r.Stderr + r.Stdout
}

func (r ExecResult) Pretty() api.Text {

	t := api.Text{Content: r.Command, Style: "italic font-mono"}

	if log.IsTraceEnabled() && r.PID > 0 {
		t = t.Space().Append("[pid:", "text-muted").Append(r.PID).Append("]", "text-muted")
	}

	// Build command string
	if len(r.Args) > 0 {
		t = t.Space().Append(strings.Join(r.Args, " "), "text-muted")
	}

	if !r.IsCompleted() {
		t = t.Space().Append(r.Status, "text-yellow-500")
	} else if !r.IsOk() {
		t = t.Space().Append(fmt.Sprintf("exit: %d", r.ExitCode), "text-red-500")
	}

	if r.Error != nil && r.Error.Error() != fmt.Sprintf("exit code: %d", r.ExitCode) {
		t = t.Space().Append("error: ", "text-muted").Append(r.Error.Error(), "text-red-500")
	}

	if r.Duration > 1*time.Second {
		t = t.Space().Append(r.Duration, "text-orange-500")
	}

	return t

}

func (r ExecResult) PrettyFull() api.Textable {

	t := r.Pretty()

	if r.Stdout != "" {
		t = t.NewLine().Append(r.Stdout, "max-lines-[20] truncate-headtail")
	}
	if r.Stderr != "" {
		t = t.NewLine().Append(r.Stderr, "text-red-500 max-lines-[20] truncate-headtail")
	}
	return t
}

// parseCommand resolves the binary and argv for the configured command. It is
// pure: it never mutates p, so it is safe to call concurrently with Run()
// (which performs its own resolution before publishing process state).
func (p *Process) parseCommand() (binary string, args []string, err error) {

	args = p.Args
	shell := p.Shell
	if shell == "" && p.Cmd != "" {
		if _, e := exec.LookPath(p.Cmd); e != nil {
			// The command is not directly executable, use bash as default shell
			shell = "bash"
		} else if ContainsShellOperators(p.Cmd) {
			// The command contains shell operators, use bash as default shell
			shell = "bash"
		}
	}

	// If Args is empty, treat Cmd as a shell command
	if shell != "" {

		args = append([]string{p.Cmd}, p.Args...)
		binary = shell

		// args = lo.Map(args, func(arg string, _ int) string {
		// 	return shellescape.Quote(arg)
		// })
		args = []string{strings.Join(args, " ")}

		switch filepath.Base(binary) {
		case "bash", "sh", "shell", "zsh", "python", "python3":
			args = append([]string{"-c"}, args...)
		case "pwsh", "powershell":
			args = append([]string{"-Command"}, args...)

			if !strings.HasSuffix(binary, "pwsh") {
				binary = "pwsh"
			}
		case "typescript", "ts", "ts-node":
			args = append([]string{"-e"}, args...)
			if !strings.HasSuffix(binary, "ts-node") {
				binary = "ts-node"
			}

		case "javascript", "js", "node":
			if !strings.HasSuffix(binary, "node") {
				binary = "node"
			}
			args = append([]string{"-e"}, args...)
		}

		if _, e := os.Stat(binary); e == nil {
			p.log.V(4).Infof("Using path %s", binary)
		} else if _path, e := exec.LookPath(binary); e == nil {
			p.log.V(5).Infof("Resolved %s to %s", binary, _path)
		} else if strings.HasSuffix(binary, "bash") {
			if _, e = exec.LookPath("sh"); e == nil {
				p.log.V(3).Infof("Using sh instead of bash")
				binary = "sh"
			} else {
				err = fmt.Errorf("shell not found: %s", binary)
				return
			}
		} else {
			err = fmt.Errorf("shell not found: %s", binary)
			return
		}
	} else {
		binary = p.Cmd
	}

	return

}

// Run executes the process and captures its output and exit status

func (p *Process) Run() *Process {
	// Close done channel when Run completes
	if p.done != nil {
		defer close(p.done)
	}

	// Close the parent-side stdio pipe ends when Run returns so they do not leak
	// across supervised restarts (the child has already exited / EOF'd by then).
	if p.wantStdioPipe {
		defer func() {
			p.mu.Lock()
			sin, sout := p.stdin, p.stdoutR
			p.stdin, p.stdoutR = nil, nil
			p.mu.Unlock()
			if sin != nil {
				_ = sin.Close()
			}
			if c, ok := sout.(io.Closer); ok && c != nil {
				_ = c.Close()
			}
		}()
	}

	if p.log == nil {
		p.log = logger.GetLogger("exec")
	}
	if p.captureOutput == nil {
		p.captureOutput = NewExecLogger()
	}
	p.captureOutput.Reset()

	if !p.exactEnvironment && (properties.On(false, "exec.debug") || strings.Contains(os.Getenv("DEBUG"), "exec")) {
		p = p.Debug()
	}

	// A context that is already canceled (or has an expired deadline) must
	// prevent the child from ever starting.
	if p.ctx != nil {
		if ctxErr := p.ctx.Err(); ctxErr != nil {
			p.mu.Lock()
			p.Err = ctxErr
			p.mu.Unlock()
			return p
		}
	}

	var cmd *exec.Cmd

	path, args, err := p.parseCommand()

	if err != nil {
		p.mu.Lock()
		p.Err = err
		p.mu.Unlock()
		return p
	}

	cmd = exec.Command(path, args...)

	if p.Cwd != "" {
		cmd.Dir = p.Cwd
	}

	cmd.Env = p.environment()

	cmd.Stderr = p.captureOutput.GetStderrWriter()

	// childStdinR / childStdoutW are the child-side ends of the stdio pipes; the
	// parent must close them right after Start() so the child gets EOF on its
	// stdin when p.stdin is closed and the parent's reader sees EOF when the
	// child exits. They stay nil unless WithStdioPipe was set.
	var childStdinR, childStdoutW *os.File
	if p.wantStdioPipe {
		var perr error
		var stdinW *os.File
		childStdinR, stdinW, perr = os.Pipe()
		if perr != nil {
			p.Err = fmt.Errorf("stdin pipe: %w", perr)
			return p
		}
		var stdoutR *os.File
		stdoutR, childStdoutW, perr = os.Pipe()
		if perr != nil {
			_ = childStdinR.Close()
			_ = stdinW.Close()
			p.Err = fmt.Errorf("stdout pipe: %w", perr)
			return p
		}
		cmd.Stdin = childStdinR
		cmd.Stdout = childStdoutW
		p.mu.Lock()
		p.stdin = stdinW
		p.stdoutR = capturingReadCloser{
			Reader: io.TeeReader(stdoutR, p.captureOutput.GetStdoutWriter()),
			Closer: stdoutR,
		}
		p.mu.Unlock()
	} else {
		cmd.Stdin = p.input
		cmd.Stdout = p.captureOutput.GetStdoutWriter()
	}

	applyProcessGroup(cmd, p.newProcessGroup)
	if p.newProcessGroup || p.wantStdioPipe {
		// Force-close the stdio pipes shortly after the direct child exits so
		// Wait() can return even when orphaned descendants (e.g. tsx→node→claude)
		// still hold them open. Go 1.20+ feature.
		cmd.WaitDelay = 2 * time.Second
	}

	p.log.Tracef("%s", api.Text{}.Append("run", "text-muted").Append(icons.MinimalArrow, "text-muted").Space().Add(p.PrettyShort()).ANSI())

	now := time.Now()
	p.mu.Lock()
	p.Started = &now
	p.cmd = cmd
	p.mu.Unlock()

	// Start (not Run) so the pid is published the moment os/exec sets
	// cmd.Process, before the blocking Wait(). This lets probes read p.pid
	// (a clicky-owned field) instead of cmd.Process, which os/exec mutates
	// without a lock we control.
	var runErr error
	if startErr := cmd.Start(); startErr != nil {
		// Close the child-side pipe ends the parent still holds; the deferred
		// cleanup only closes the parent-side ends, so without this the two
		// child-side descriptors leak on every failed start (e.g. a restart
		// policy retrying a bad path).
		if childStdinR != nil {
			_ = childStdinR.Close()
		}
		if childStdoutW != nil {
			_ = childStdoutW.Close()
		}
		p.mu.Lock()
		p.Err = startErr
		p.mu.Unlock()
		return p
	}
	p.mu.Lock()
	p.running = true
	if cmd.Process != nil {
		p.pid = cmd.Process.Pid
	}
	p.mu.Unlock()

	// Close the parent's copies of the child-side pipe ends now that the child
	// has inherited them: without this the parent's StdoutReader never sees EOF
	// (the parent still holds a write end) and the child never sees stdin EOF.
	if childStdinR != nil {
		_ = childStdinR.Close()
	}
	if childStdoutW != nil {
		_ = childStdoutW.Close()
	}

	// Wait for completion. The lock is intentionally NOT held across Wait():
	// probes (IsRunning/Pid/KillTree) must stay responsive for the whole
	// subprocess lifetime. Run() is the ONLY reaper of the child: the
	// goroutine below owns the sole cmd.Wait() call, and the timeout /
	// cancellation branches drain resultChan instead of waiting on the
	// process themselves.
	resultChan := make(chan error, 1)
	go func() {
		resultChan <- cmd.Wait()
	}()

	var timeoutChan <-chan time.Time
	if p.Timeout > 0 {
		timer := time.NewTimer(p.Timeout)
		defer timer.Stop()
		timeoutChan = timer.C
	}
	var ctxDone <-chan struct{}
	if p.ctx != nil {
		ctxDone = p.ctx.Done()
	}

	select {
	case err := <-resultChan:
		runErr = err
	case <-timeoutChan:
		p.stopAndReap(resultChan, 5*time.Second)
		runErr = fmt.Errorf("command timed out after %v", api.Human(time.Since(*p.Started).String()))
	case <-ctxDone:
		p.stopAndReap(resultChan, 5*time.Second)
		runErr = p.ctx.Err()
	}

	// Classify the error before publishing. Reading ProcessState here is safe:
	// cmd.Wait() has returned, so os/exec is done mutating it.
	var exitCode *int
	completed, succeeded := false, false
	if cmd.ProcessState != nil {
		exitCode = lo.ToPtr(cmd.ProcessState.ExitCode())
		completed = true
		succeeded = cmd.ProcessState.Success()
	}
	if runErr != nil {
		p.log.Debugf("%s", api.Text{}.Add(p.PrettyShort()).Append(" finished with ").Append(runErr, "text-red-500").ANSI())
	}
	if v, ok := runErr.(*exec.ExitError); ok {
		switch {
		case p.SucceedOnNonZero:
			// non-zero exit codes are not considered errors
			runErr = nil
		case exitCode != nil:
			runErr = fmt.Errorf("exit code: %d", *exitCode)
		}
		// Enhance fork/exec permission errors with better messages
		if v.ExitCode() == 126 {
			runErr = fmt.Errorf("permission denied: %s", p.Cmd)
		}
		if v.ExitCode() == 127 {
			runErr = fmt.Errorf("command not found: %s", p.Cmd)
		}
	}

	// Publish terminal state under the lock so concurrent readers observe a
	// consistent (running=false, Err, exitCode, exited, succeeded) tuple.
	p.mu.Lock()
	p.running = false
	p.Err = runErr
	p.exitCode = exitCode
	p.completed = completed
	p.succeeded = succeeded
	p.mu.Unlock()

	p.log.Tracef("%s", p.Pretty().ANSI())

	return p
}
