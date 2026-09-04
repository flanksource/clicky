package exec

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/shutdown"
	"github.com/flanksource/clicky/task"
	"github.com/flanksource/commons/logger"
	"github.com/samber/lo"
)

func (p *Process) Result() *ExecResult {
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

	result := &ExecResult{
		Stdout:   p.captureOutput.GetStdout(),
		Stderr:   p.captureOutput.GetStderr(),
		ExitCode: exitCode,
		Error:    procErr,
		Started:  started,
		Command:  p.Cmd,
		Args:     p.Args,
		process:  p,
	}
	// A non-nil Err with completed=false means the process never started
	// (parse or start failure) — that is terminal, not pending.
	switch {
	case procErr == nil && completed:
		result.Status = "success"
	case procErr == nil:
		result.Status = "pending"
	case strings.Contains(procErr.Error(), "timed out"):
		result.Status = "timeout"
	case strings.Contains(procErr.Error(), "permission denied"):
		result.Status = "permission denied"
	default:
		result.Status = "failed"
	}
	result.PID = pid
	if started != nil {
		result.Duration = time.Since(*started)
	}
	return result
}

func (e *ExecResult) Refresh() *ExecResult { return e.process.Result() }
func (p *Process) Out() string             { return p.captureOutput.GetOutput() }
func (p *Process) Pretty() api.Text        { return p.Result().Pretty() }

func (p *Process) WithEnv(env map[string]string) *Process {
	if p.Env == nil {
		p.Env = make(map[string]string, len(env))
	}
	for key, value := range env {
		p.Env[key] = value
	}
	return p
}

func (p *Process) WithCwd(cwd string) *Process {
	p.Cwd = cwd
	return p
}

func (p *Process) WithTask(task *task.Task) *Process {
	p.task = task
	return p
}

func (p *Process) WithLogger(log logger.Logger) *Process {
	p.log = log
	return p
}

func (p *Process) WithTimeout(timeout time.Duration) *Process {
	p.Timeout = timeout
	return p
}

// WithCaptureLimit bounds each captured stdout and stderr snapshot to the
// newest maxBytes. Stream and WithStdioPipe destinations still receive every
// byte. Processes remain unbounded when this option is not used.
func (p *Process) WithCaptureLimit(maxBytes int) *Process {
	if maxBytes <= 0 {
		panic("capture limit must be positive")
	}
	if p.captureOutput == nil {
		p.captureOutput = NewExecLogger()
	}
	p.captureOutput.setCaptureLimit(maxBytes)
	return p
}

func (p *Process) WithProcessGroup() *Process {
	p.newProcessGroup = true
	return p
}

func (p *Process) Pid() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pid
}

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

// Start launches the process in the background as a managed clicky task so it
// has an identity, cancellation, and status reporting (retrieve it via
// GetTask()). The task is marked background so global waits skip it; use
// Stop/MustStop (also wired as a shutdown hook) to terminate it.
func (p *Process) Start() error {
	shutdown.AddHook("Stopping "+p.Name(), func() { _ = p.MustStop(10 * time.Second) })
	started := p.StartAsTask(p.Name())
	started.SetBackground(true)
	return nil
}

// MustStop interrupts the process and escalates to SIGKILL when it has not
// exited within timeout, so a child that ignores SIGINT is still reaped.
func (p *Process) MustStop(timeout time.Duration) error { return p.Kill(timeout) }
func (p *Process) Stop() error                          { return p.Terminate() }
func (p *Process) GetOutput() string                    { return p.captureOutput.GetOutput() }
func (p *Process) GetStdout() string                    { return p.captureOutput.GetStdout() }
func (p *Process) GetStderr() string                    { return p.captureOutput.GetStderr() }

func (p *Process) Stream(stdout, stderr io.Writer) *Process {
	if p.captureOutput == nil {
		p.captureOutput = NewExecLogger()
	}
	p.captureOutput = p.captureOutput.Tee(stdout, stderr)
	return p
}

func (p *Process) WithStdin(input io.Reader) *Process {
	p.input = input
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
	// Tee through the logger (logger.Verbose is an io.Writer), never directly
	// to os.Stdout/os.Stderr, so the task renderer's terminal stays intact.
	p.captureOutput = p.captureOutput.Tee(p.log.V(logger.Debug), p.log.V(logger.Debug))
	return p
}

// commandLabel is the display name of the configured command. It reads only
// caller-configured fields, so it never races with process startup.
func (p *Process) commandLabel() string {
	return lo.CoalesceOrEmpty(p.Shell, p.Cmd)
}

// PrettyShort returns a compact single-line summary of the command. It is
// pure: parseCommand does not mutate the Process, so rendering is safe
// concurrently with Run() publishing process state.
func (p *Process) PrettyShort() api.Textable {
	path, args, _ := p.parseCommand()
	text := api.Text{}.Append(lo.CoalesceOrEmpty(p.Shell, path), "font-bold text-orange-600")
	p.mu.RLock()
	pid := p.pid
	p.mu.RUnlock()
	if pid != 0 {
		text = text.Space().Append("[", "text-muted").Append(pid).Append("]", "text-muted")
	}
	if p.Shell != "" {
		text = text.Space().Append(p.Shell)
	}
	if len(args) > 0 {
		text = text.Space().Append("[", "text-muted")
		for _, arg := range args {
			text = text.Append(arg, "max-w-[100ch] truncate").Append(",", "text-muted")
		}
		text = text.Append("]", "text-muted")
	}
	if p.log != nil && p.log.V(1).Enabled() && p.Cwd != "" {
		wd, _ := os.Getwd()
		if p.Cwd != wd {
			rel, _ := filepath.Rel(wd, p.Cwd)
			if strings.HasPrefix(rel, "..") {
				rel = p.Cwd
			}
			text = text.Space().Append(fmt.Sprintf("(cwd: %s)", rel), "text-muted")
		}
	}
	return text
}
