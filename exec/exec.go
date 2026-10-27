package exec

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	duration  time.Duration

	Started *time.Time
	cmd     *exec.Cmd
	task    *task.Task
	// ctx, when set (see WithContext), cancels an in-flight Run(): an
	// already-expired context prevents the start, and ctx.Done() firing during
	// execution interrupts the child (escalating to SIGKILL) while Run itself
	// reaps it.
	ctx            context.Context
	Timeout        time.Duration
	niceness       *int
	resourceLimits ResourceLimits
	Env            map[string]string
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
	// newProcessGroup, when true, spawns the subprocess as the leader of its own
	// POSIX session and process group (Setsid), detached from the controlling
	// terminal, / Windows process group (CREATE_NEW_PROCESS_GROUP) so the whole
	// descendant tree can be terminated atomically via KillTree.
	newProcessGroup bool
	// keepTerminal narrows newProcessGroup to Setpgid: the child keeps the
	// caller's session and controlling terminal (WithControllingTerminal).
	keepTerminal bool

	// wantStdioPipe, when true, connects the child's stdin to a writable pipe
	// (Stdin) and its stdout to a readable pipe (StdoutReader) for bidirectional
	// line-protocol traffic (e.g. JSON-RPC). The pipe is tee'd into captureOutput
	// on the way past, so the transport is still readable as output; a long-lived
	// server does not accumulate unboundedly because Supervise applies a default
	// WithCaptureLimit, not because the capture is skipped. stdin/stdoutR are the
	// parent-side ends, published under mu once Run() has started the child.
	wantStdioPipe bool
	input         io.Reader
	extraFiles    []*os.File
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
	if p.done != nil {
		defer close(p.done)
	}
	// Close the parent-side stdio pipe ends when Run returns so they do not leak
	// across supervised restarts (the child has already exited / EOF'd by then).
	if p.wantStdioPipe {
		defer p.closeParentStdio()
	}
	p.prepareRun()

	// A context that is already canceled (or has an expired deadline) must
	// prevent the child from ever starting.
	if p.ctx != nil && p.ctx.Err() != nil {
		p.setErr(p.ctx.Err())
		publishExec(p, ExecExit)
		return p
	}

	cmd, child, err := p.buildCmd()
	if err != nil {
		p.setErr(err)
		publishExec(p, ExecExit)
		return p
	}

	p.log.Tracef("%s", api.Text{}.Append("run", "text-muted").Append(icons.MinimalArrow, "text-muted").Space().Add(p.PrettyShort()).ANSI())

	started := time.Now()
	p.mu.Lock()
	p.Started = &started
	p.cmd = cmd
	p.mu.Unlock()

	launch, err := p.startCmd(cmd, child)
	if err != nil {
		p.setErr(err)
		publishExec(p, ExecExit)
		return p
	}
	publishExec(p, ExecStart)
	p.finishRun(cmd, launch, p.waitCmd(cmd), started)
	return p
}

// childStdio holds the child-side ends of the stdio pipes; the parent must
// close them right after Start() so the child gets EOF on its stdin when
// p.stdin is closed and the parent's reader sees EOF when the child exits.
// Both stay nil unless WithStdioPipe was set.
type childStdio struct {
	stdinR, stdoutW *os.File
}

func (c childStdio) close() {
	if c.stdinR != nil {
		_ = c.stdinR.Close()
	}
	if c.stdoutW != nil {
		_ = c.stdoutW.Close()
	}
}

func (p *Process) setErr(err error) {
	p.mu.Lock()
	p.Err = err
	p.mu.Unlock()
}

func (p *Process) closeParentStdio() {
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
}

func (p *Process) prepareRun() {
	if p.log == nil {
		p.log = logger.GetLogger("exec")
	}
	if p.captureOutput == nil {
		p.captureOutput = NewExecLogger()
	}
	p.captureOutput.Reset()

	if !p.exactEnvironment && (properties.On(false, "exec.debug") || strings.Contains(os.Getenv("DEBUG"), "exec")) {
		p.Debug()
	}
}

func (p *Process) buildCmd() (*exec.Cmd, childStdio, error) {
	path, args, err := p.parseCommand()
	if err != nil {
		return nil, childStdio{}, err
	}
	// The launcher's status descriptor follows the caller's extra files, so
	// their child descriptor numbers are unchanged.
	path, args, err = p.runtimeCommand(path, args, 3+len(p.extraFiles))
	if err != nil {
		return nil, childStdio{}, err
	}
	cmd := exec.Command(path, args...)
	if p.Cwd != "" {
		cmd.Dir = p.Cwd
	}
	cmd.Env = p.environment()
	cmd.ExtraFiles = p.extraFiles
	cmd.Stderr = p.captureOutput.GetStderrWriter()

	child, err := p.attachStdio(cmd)
	if err != nil {
		return nil, childStdio{}, err
	}

	applyProcessGroup(cmd, p.newProcessGroup, p.keepTerminal)
	if p.newProcessGroup || p.wantStdioPipe || p.Timeout > 0 {
		// Force-close the stdio pipes shortly after the direct child exits so
		// Wait() can return even when orphaned descendants (e.g. tsx→node→claude,
		// or a daemon started by `pg_ctl start`) still hold them open; a timed
		// run otherwise blocks in stopAndReap until the descendant exits.
		// Go 1.20+ feature.
		cmd.WaitDelay = 2 * time.Second
	}
	return cmd, child, nil
}

func (p *Process) attachStdio(cmd *exec.Cmd) (childStdio, error) {
	if !p.wantStdioPipe {
		cmd.Stdin = p.input
		cmd.Stdout = p.captureOutput.GetStdoutWriter()
		return childStdio{}, nil
	}
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		return childStdio{}, fmt.Errorf("stdin pipe: %w", err)
	}
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		_ = stdinR.Close()
		_ = stdinW.Close()
		return childStdio{}, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stdin = stdinR
	cmd.Stdout = stdoutW
	p.mu.Lock()
	p.stdin = stdinW
	p.stdoutR = capturingReadCloser{
		Reader: io.TeeReader(stdoutR, p.captureOutput.GetStdoutWriter()),
		Closer: stdoutR,
	}
	p.mu.Unlock()
	return childStdio{stdinR: stdinR, stdoutW: stdoutW}, nil
}

// startCmd uses Start (not Run) so the pid is published the moment os/exec
// sets cmd.Process, before the blocking Wait(). This lets probes read p.pid
// (a clicky-owned field) instead of cmd.Process, which os/exec mutates
// without a lock we control.
func (p *Process) startCmd(cmd *exec.Cmd, child childStdio) (*launchStatus, error) {
	var launch *launchStatus
	var err error
	if p.usesLauncher() {
		launch, err = newLaunchStatus()
	}
	if launch != nil {
		cmd.ExtraFiles = append(slices.Clone(p.extraFiles), launch.writer)
	}
	if err == nil {
		err = cmd.Start()
	}
	if launch != nil {
		launch.started()
	}
	if err != nil {
		if launch != nil {
			_ = launch.reader.Close()
		}
		// The deferred cleanup only closes the parent-side ends, so without this
		// the child-side descriptors leak on every failed start.
		child.close()
		return nil, err
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
	child.close()
	return launch, nil
}

// waitCmd does not hold the lock across Wait(): probes (IsRunning/Pid/KillTree)
// must stay responsive for the whole subprocess lifetime. Run() is the ONLY
// reaper of the child: the goroutine below owns the sole cmd.Wait() call, and
// the timeout / cancellation branches drain resultChan instead of waiting on
// the process themselves.
func (p *Process) waitCmd(cmd *exec.Cmd) error {
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
	return p.waitRuntime(resultChan, timeoutChan, ctxDone)
}

func (p *Process) finishRun(cmd *exec.Cmd, launch *launchStatus, runErr error, started time.Time) {
	finished := time.Now()

	// Reading ProcessState here is safe: cmd.Wait() has returned, so os/exec is
	// done mutating it.
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
	runErr = p.classifyRunErr(runErr, exitCode, launch)

	// Publish terminal state under the lock so concurrent readers observe a
	// consistent (running=false, Err, exitCode, exited, succeeded) tuple.
	p.mu.Lock()
	p.running = false
	p.Err = runErr
	p.exitCode = exitCode
	p.completed = completed
	p.succeeded = succeeded
	p.duration = finished.Sub(started)
	p.mu.Unlock()

	publishExec(p, ExecExit)
	p.log.Tracef("%s", p.Pretty().ANSI())
}

func (p *Process) classifyRunErr(runErr error, exitCode *int, launch *launchStatus) error {
	if v, ok := runErr.(*exec.ExitError); ok {
		switch {
		case p.SucceedOnNonZero:
			runErr = nil
		case exitCode != nil:
			runErr = fmt.Errorf("exit code: %d: %w", *exitCode, v)
		}
		if v.ExitCode() == 126 {
			runErr = fmt.Errorf("permission denied: %s", p.Cmd)
		}
		if v.ExitCode() == 127 {
			runErr = fmt.Errorf("command not found: %s", p.Cmd)
		}
	}
	// The launcher exits before the target runs, so its failure is not a target
	// exit status and SucceedOnNonZero must not clear it.
	if launch != nil {
		if launchErr := launch.err(); launchErr != nil {
			return launchErr
		}
	}
	return runErr
}
