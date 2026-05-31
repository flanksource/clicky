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
	"github.com/flanksource/clicky/shutdown"
	"github.com/flanksource/clicky/task"
	cctx "github.com/flanksource/commons/context"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/properties"
	"github.com/samber/lo"
)

var log = logger.GetLogger("exec")

// NewExecf creates a new Process with formatted command string
func NewExecf(cmd string, args ...any) *Process {
	return &Process{
		Cmd:  fmt.Sprintf(cmd, args...),
		log:  log,
		Env:  map[string]string{},
		done: make(chan struct{}),
	}
}

// NewExec creates a new Process with the specified command and arguments
func NewExec(cmd string, args ...string) *Process {
	return &Process{
		Cmd:  cmd,
		log:  log,
		Args: args,
		Env:  map[string]string{},
		done: make(chan struct{}),
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
	Timeout time.Duration
	Env     map[string]string
	Cwd     string
	Err     error
	Shell   string
	Cmd     string
	Args    []string
	// Consider a non-zero exit code as an error
	SucceedOnNonZero bool
	exitCode         *int
	done             chan struct{} // Closed when Run() completes
	// newProcessGroup, when true, spawns the subprocess in its own POSIX
	// process group (Setpgid) / Windows process group (CREATE_NEW_PROCESS_GROUP)
	// so the whole descendant tree can be terminated atomically via KillTree.
	newProcessGroup bool
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
	short    api.Text      `json:"-"`
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

// WrapperFunc is a function type returned by AsWrapper that executes commands
// with pre-configured settings from a template Process.
type WrapperFunc func(...any) (*ExecResult, error)

// WrapperOption is a functional option that modifies a Process for a single execution.
type WrapperOption interface {
	apply(*Process)
}

type wrapperOptionFunc func(*Process)

func (f wrapperOptionFunc) apply(p *Process) {
	f(p)
}

// WithTimeout returns a WrapperOption that overrides the execution timeout.
func WithTimeout(timeout time.Duration) WrapperOption {
	return wrapperOptionFunc(func(p *Process) {
		p.Timeout = timeout
	})
}

// WithContext returns a WrapperOption that sets a context for cancellation/deadline.
// Note: Currently not fully implemented for context-based cancellation.
func WithContext(ctx context.Context) WrapperOption {
	return wrapperOptionFunc(func(p *Process) {
		if deadline, ok := ctx.Deadline(); ok {
			p.Timeout = time.Until(deadline)
		}
	})
}

func WithTee(stdout, stderr io.Writer) WrapperOption {
	return wrapperOptionFunc(func(p *Process) {
		if p.captureOutput == nil {
			p.captureOutput = NewExecLogger()
		}
		p.captureOutput.Tee(stdout, stderr)
	})
}

func WithLogger(log logger.Logger) WrapperOption {
	return wrapperOptionFunc(func(p *Process) {
		p.log = log
	})
}

// WithEnv returns a WrapperOption that adds or overrides an environment variable.
func WithEnv(key, value string) WrapperOption {
	return wrapperOptionFunc(func(p *Process) {
		if p.Env == nil {
			p.Env = make(map[string]string)
		}
		p.Env[key] = value
	})
}

// WithDir returns a WrapperOption that overrides the working directory.
func WithDir(path string) WrapperOption {
	return wrapperOptionFunc(func(p *Process) {
		p.Cwd = path
	})
}

// WithoutErrorOnNonZero returns a WrapperOption that makes non-zero exit codes succeed
func WithoutErrorOnNonZero() WrapperOption {
	return wrapperOptionFunc(func(p *Process) {
		p.SucceedOnNonZero = true
	})
}

func WithDebug() WrapperOption {
	return wrapperOptionFunc(func(p *Process) {
		p.Debug()
	})
}

func (p *Process) clone() *Process {
	cloned := &Process{
		Cwd:              p.Cwd,
		Cmd:              p.Cmd,
		Args:             make([]string, len(p.Args)),
		Timeout:          p.Timeout,
		captureOutput:    p.captureOutput,
		SucceedOnNonZero: p.SucceedOnNonZero,
		log:              p.log,
		Shell:            p.Shell,
		done:             make(chan struct{}),
	}

	if p.Env != nil {
		cloned.Env = make(map[string]string, len(p.Env))
		for k, v := range p.Env {
			cloned.Env[k] = v
		}
	}

	copy(cloned.Args, p.Args)
	return cloned
}

// AsWrapper converts the Process into a reusable WrapperFunc that executes commands
// with the template's configuration. Each invocation creates a new Process instance
// by copying the template's settings.
//
// Example:
//
//	docker := clicky.Exec("docker", "-v").AsWrapper()
//	result, err := docker("ps", "-a")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.Stdout)
func (p *Process) AsWrapper() WrapperFunc {
	return func(args ...any) (*ExecResult, error) {
		var stringArgs []string

		newProc := p.clone()

		for _, arg := range args {
			switch v := arg.(type) {
			case WrapperOption:
				v.apply(newProc)
			case string:
				stringArgs = append(stringArgs, v)
			default:
				stringArgs = append(stringArgs, fmt.Sprintf("%v", v))
			}
		}

		newProc.Args = append(newProc.Args, stringArgs...)

		result := newProc.Run()
		res := result.Result()

		// Return the actual error if there is one, or check exit code
		if result.Err != nil {
			return res, result.Err
		}

		// Return error if ErrorOnNonZero is not set and exit code is non-zero
		if !newProc.SucceedOnNonZero && res.ExitCode != 0 {
			return res, fmt.Errorf("exit code %d", res.ExitCode)
		}

		return res, nil
	}
}

func (p *Process) WithoutShell() *Process {
	p.Shell = ""
	return p
}

func (p *Process) WithShell(shell string) *Process {
	p.Shell = shell
	return p
}

func (p *Process) Result() *ExecResult {
	// Snapshot all mutable state in a single critical section so the result is
	// internally consistent even if Run() is publishing terminal state
	// concurrently. captureOutput, Cmd and Args are not mutated after Run
	// begins, so they're read outside the lock.
	p.mu.RLock()
	procErr := p.Err
	started := p.Started
	pid := p.pid
	completed := p.completed
	exitCode := -1
	if p.exitCode != nil {
		exitCode = *p.exitCode
	}
	p.mu.RUnlock()

	r := &ExecResult{
		Stdout:   p.captureOutput.GetStdout(),
		Stderr:   p.captureOutput.GetStderr(),
		ExitCode: exitCode,
		Error:    procErr,
		Started:  started,
		Command:  p.Cmd,
		Args:     p.Args,
		short:    p.Short(),
		process:  p,
	}

	if completed {
		if procErr == nil {
			r.Status = "success"
		} else if strings.Contains(procErr.Error(), "timed out") {
			r.Status = "timeout"
		} else if strings.Contains(procErr.Error(), "permission denied") {
			r.Status = "permission denied"
		} else {
			r.Status = "failed"
		}
	} else {
		r.Status = "pending"
	}

	r.PID = pid

	if started != nil {
		r.Duration = time.Since(*started)
	}
	return r
}

// Refresh refreshes the ExecResult by re-fetching data from the underlying Process
func (e *ExecResult) Refresh() *ExecResult {
	return e.process.Result()
}

func (p *Process) Out() string {
	return p.captureOutput.GetOutput()
}

func (p *Process) Pretty() api.Text {
	return p.Result().Pretty()
}

func (p *Process) WithEnv(env map[string]string) *Process {
	p.Env = env
	return p
}

func (p *Process) WithCwd(cwd string) *Process {
	p.Cwd = cwd
	return p
}

func (p *Process) WithTask(t *task.Task) *Process {
	p.task = t
	return p
}

func (p *Process) WithTimeout(timeout time.Duration) *Process {
	p.Timeout = timeout
	return p
}

// WithProcessGroup spawns the subprocess in its own process group so the
// entire descendant tree can be terminated atomically via KillTree. Must be
// called before Run/Start. On POSIX this sets Setpgid=true; on Windows it
// sets CREATE_NEW_PROCESS_GROUP.
func (p *Process) WithProcessGroup() *Process {
	p.newProcessGroup = true
	return p
}

// Pid returns the OS pid of the running subprocess, or 0 if it hasn't
// started yet. Safe to call concurrently with Run().
func (p *Process) Pid() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pid
}

// KillTree terminates the subprocess and every descendant. Prefer this over
// Kill/ForceKill for processes that fork or exec grandchildren (test runners,
// headless browsers, etc). When WithProcessGroup was set at spawn, the kill
// is atomic (POSIX: SIGKILL to -pgid; Windows: TerminateJobObject); otherwise
// KillTree falls back to a gopsutil-style descendant walk, which is racy.
func (p *Process) KillTree() error {
	p.mu.RLock()
	pid, newGroup := p.pid, p.newProcessGroup
	p.mu.RUnlock()
	if pid == 0 {
		return nil
	}
	return killTree(pid, newGroup)
}

func (p *Process) Name() string {
	p.mu.RLock()
	cmd := p.cmd
	p.mu.RUnlock()
	if cmd != nil {
		return cmd.Path
	}
	return p.Cmd
}

// Start runs the process in the background
func (p *Process) Start() error {
	shutdown.AddHook("Stopping "+p.Name(), func() {
		_ = p.MustStop(10 * time.Second) // ignore error during shutdown
	})
	go p.Run()
	return nil
}

// MustStop attempts to gracefully stop a process, after which it is forcefully killed
func (p *Process) MustStop(timeout time.Duration) error {
	if err := p.Terminate(); err != nil {
		return err
	}
	return nil
}

func (p *Process) Stop() error {
	return p.Terminate()
}

func (p *Process) GetOutput() string {
	return p.captureOutput.GetOutput()
}

func (p *Process) GetStdout() string {
	return p.captureOutput.GetStdout()
}

func (p *Process) GetStderr() string {
	return p.captureOutput.GetStderr()
}

func (p *Process) Stream(stdout, stderr io.Writer) *Process {
	if p.captureOutput == nil {
		p.captureOutput = NewExecLogger()
	}
	p.captureOutput.Tee(stdout, stderr)
	return p
}

func (p *Process) Debug() *Process {
	if p.log == nil {
		p.log = logger.GetLogger("exec")
	}
	p.log = NewDebugLogger(p.log, logger.Trace)
	if p.captureOutput == nil {
		p.captureOutput = NewExecLogger()
	}
	p.captureOutput = p.captureOutput.Tee(os.Stdout, os.Stderr)
	return p
}

func (p *Process) Short() api.Text {
	path, args, _ := p.parseCommand()

	t := api.Text{Content: lo.CoalesceOrEmpty(p.Shell, path), Style: "font-bold text-orange-600"}

	p.mu.RLock()
	pid := p.pid
	p.mu.RUnlock()
	if pid != 0 {
		t = t.Space().Append("[", "text-muted").Append(pid).Append("]", "text-muted")
	}
	if p.Shell != "" {
		t = t.Space().Append(p.Shell)
	}
	if len(args) > 0 {
		t = t.Space().Append("[", "text-muted")
		for _, arg := range args {
			t = t.Append(arg, "max-w-[100ch] truncate").Append(",", "text-muted")
		}
		t = t.Append("]", "text-muted")
	}

	if p.log.V(1).Enabled() && p.Cwd != "" {
		wd, _ := os.Getwd()
		if p.Cwd != wd {
			rel, _ := filepath.Rel(wd, p.Cwd)
			if strings.HasPrefix(rel, "..") {
				rel = p.Cwd
			}
			t = t.Space().Append(fmt.Sprintf("(cwd: %s)", rel), "text-muted")
		}
	}

	return t
}

func (p *Process) parseCommand() (binary string, args []string, err error) {

	args = p.Args
	if p.Shell == "" && p.Cmd != "" {
		if _, e := exec.LookPath(p.Cmd); e != nil {
			// The command is not directly executable, use bash as default shell
			p.Shell = "bash"
		} else if ContainsShellOperators(p.Cmd) {
			// The command contains shell operators, use bash as default shell
			p.Shell = "bash"
		}
	}

	// If Args is empty, treat Cmd as a shell command
	if p.Shell != "" {

		args = append([]string{p.Cmd}, p.Args...)
		binary = p.Shell

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

	if p.log == nil {
		p.log = logger.GetLogger("exec")
	}
	if p.captureOutput == nil {
		p.captureOutput = NewExecLogger()
	}
	p.captureOutput.Reset()

	if properties.On(false, "exec.debug") || strings.Contains(os.Getenv("DEBUG"), "exec") {
		p = p.Debug()
	}

	var cmd *exec.Cmd

	path, args, err := p.parseCommand()

	if err != nil {
		p.Err = err
		return p
	}

	cmd = exec.Command(path, args...)

	if p.Cwd != "" {
		cmd.Dir = p.Cwd
	}

	// Set environment variables
	cmd.Env = os.Environ() // Start with current environment
	for k, v := range p.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))

	}

	cmd.Stderr = p.captureOutput.GetStderrWriter()
	cmd.Stdout = p.captureOutput.GetStdoutWriter()

	applyProcessGroup(cmd, p.newProcessGroup)
	if p.newProcessGroup {
		// Force-close the stdout/stderr pipes 2s after the direct child
		// exits so Wait() can return even when orphaned descendants still
		// hold them open. Go 1.20+ feature.
		cmd.WaitDelay = 2 * time.Second
	}

	p.log.Tracef(api.Text{}.Append("run", "text-muted").Append(icons.MinimalArrow, "text-muted").Space().Add(p.Short()).ANSI())

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

	// Wait for completion. The lock is intentionally NOT held across Wait():
	// probes (IsRunning/Pid/KillTree) must stay responsive for the whole
	// subprocess lifetime.
	if p.Timeout > 0 {
		resultChan := make(chan error, 1)
		go func() {
			resultChan <- cmd.Wait()
		}()

		select {
		case err := <-resultChan:
			runErr = err
		case <-time.After(p.Timeout):
			_ = p.Kill(5 * time.Second)
			// Wait for the Wait() goroutine to return so os/exec has finished
			// mutating cmd.ProcessState before we read it below — otherwise the
			// classification read races the in-flight Wait().
			<-resultChan
			runErr = fmt.Errorf("command timed out after %v", api.Human(time.Since(*p.Started).String()))
		}
	} else {
		runErr = cmd.Wait()
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
		p.log.Debugf(p.Short().Append(" finished with ").Append(runErr, "text-red-500").ANSI())
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

	p.log.Tracef(p.Pretty().ANSI())

	return p
}

func (p *Process) ExitCode() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.exitCode != nil {
		return *p.exitCode
	}
	return -1
}

func (p *Process) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	// running is the authoritative signal: it is true only while the
	// subprocess is executing. Peeking cmd.ProcessState would race, since
	// os/exec publishes it without synchronization a caller can observe.
	return p.running && p.pid != 0
}

func (p *Process) IsOK() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Err == nil && p.completed && p.succeeded
}

func (p *Process) Wait() error {
	p.mu.RLock()
	cmd := p.cmd
	p.mu.RUnlock()
	if cmd == nil {
		return nil
	}
	return cmd.Wait()
}

func (p *Process) Terminate() error {
	if !p.IsRunning() {
		return nil
	}
	p.log.Infof(api.Text{}.Add(icons.Stop).Space().Append("Terminating process: ").Add(p.Short()).ANSI())

	p.mu.RLock()
	proc := p.cmd.Process
	p.mu.RUnlock()

	// Send interrupt signal
	if err := proc.Signal(os.Interrupt); err != nil {
		p.log.Errorf(api.Text{}.Add(icons.Fail).Space().Append(" failed to signal ").Add(p.Short()).Append(err, "text-red-500").ANSI())
		return err
	}

	// Wait for Run() to complete and update ProcessState
	if p.done != nil {
		select {
		case <-p.done:
			// Run() completed successfully, ProcessState is now updated
			p.log.Tracef(api.Text{}.Append("terminated ").Add(p.Short()).ANSI())
		case <-time.After(5 * time.Second):
			p.log.Warnf("Timeout waiting for Run() to complete after termination")
			return fmt.Errorf("timeout waiting for process termination")
		}
	}

	return nil
}

func (p *Process) Kill(timeout time.Duration) error {
	if !p.IsRunning() {
		return nil
	}

	p.log.Infof(api.Text{}.Add(icons.Zombie).Space().Append("Killing process: ").Add(p.Short()).ANSI())

	p.log.Infof("Killing process: %s with timeout %v", p.Short().ANSI(), timeout)

	p.mu.RLock()
	proc := p.cmd.Process
	p.mu.RUnlock()

	if err := proc.Signal(os.Interrupt); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		_, err := proc.Wait()
		done <- err
	}()

	select {
	case <-time.After(timeout):
		return p.ForceKill()
	case err := <-done:
		return err
	}
}

func (p *Process) ForceKill() error {
	p.mu.RLock()
	cmd := p.cmd
	p.mu.RUnlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

// WaitForStdout waits for a specific message to appear in the process stdout
// This is useful for waiting for server startup messages like "Server started on port"
func (p *Process) WaitForStdout(message string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Check if process has exited with error
		p.mu.RLock()
		failed := p.completed && !p.succeeded
		procErr := p.Err
		p.mu.RUnlock()
		if failed {
			return fmt.Errorf("process exited with error: %v", procErr)
		}

		// Check stdout buffer for the message
		stdout := p.GetStdout()
		if strings.Contains(stdout, message) {
			return nil
		}

		// Brief sleep to avoid busy waiting
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting '%s' in stdout after %v", message, timeout)
}

// GetTask implements the Taskable interface
func (p *Process) GetTask() *task.Task {
	return p.task
}

// StartAsTask creates and starts a Task for this Process with typed result handling
func (p *Process) RunAsTask(name string, opts ...task.Option) task.TypedTask[ExecResult] {
	taskFunc := func(ctx cctx.Context, t *task.Task) (ExecResult, error) {
		p.task = t
		out := p.Run()
		p = out

		err := p.Err
		// Return the result and error
		return *out.Result(), err
	}

	return task.StartTask(name, taskFunc, opts...)

}

// StartAsTask creates and starts a Task for this Process with typed result handling
func (p *Process) StartAsTask(name string, opts ...task.Option) task.TypedTask[*Process] {
	taskFunc := func(ctx cctx.Context, t *task.Task) (*Process, error) {
		p.task = t
		// Run the process
		p = p.Run()

		err := p.Err
		// Return the result and error
		return p, err
	}

	return task.StartTask(name, taskFunc, opts...)
}
