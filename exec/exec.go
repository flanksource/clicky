package exec

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/shutdown"
	"github.com/flanksource/clicky/task"
	cctx "github.com/flanksource/commons/context"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/properties"
	"github.com/onsi/ginkgo/v2"
	"github.com/samber/lo"
)

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

	Started *time.Time
	cmd     *exec.Cmd
	task    *task.Task
	Timeout time.Duration
	Env     map[string]string
	Cwd     string
	Err     error
	Cmd     string
	Args    []string
	// Consider a non-zero exit code as an error
	ErrorOnNonZero bool
	exitCode       *int
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

	t := api.Text{Content: r.Command, Style: api.Clz(r.IsOk(), "text-green-500")}

	if !r.IsCompleted() {
		t = t.Space().Append(r.Status, "text-yellow-500")
	} else if !r.IsOk() {
		t = t.Space().Append(fmt.Sprintf("exit: %d", r.ExitCode), "text-red-500")

	}

	if r.Duration > 1*time.Second {
		t = t.Space().Append(r.Duration, "text-orange-500")
	}
	if r.Error != nil {
		t = t.Space().Append("error: ", "text-muted").Append(r.Error.Error(), "text-red-500")
	}

	// Build command string
	if len(r.Args) > 0 {
		t = t.Space().Append(strings.Join(r.Args, " "), "text-muted")
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

// WithErrOnNonZero returns a WrapperOption that makes non-zero exit codes return an error.
func WithErrOnNonZero() WrapperOption {
	return wrapperOptionFunc(func(p *Process) {
		p.ErrorOnNonZero = true
	})
}

func (p Process) clone() Process {
	cloned := Process{
		Cwd:            p.Cwd,
		Cmd:            p.Cmd,
		Args:           make([]string, len(p.Args)),
		Timeout:        p.Timeout,
		captureOutput:  p.captureOutput,
		ErrorOnNonZero: p.ErrorOnNonZero,
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
func (p Process) AsWrapper() WrapperFunc {
	return func(args ...any) (*ExecResult, error) {
		var stringArgs []string

		newProc := p.clone()

		for _, arg := range args {
			switch v := arg.(type) {
			case WrapperOption:
				v.apply(&newProc)
			case string:
				stringArgs = append(stringArgs, v)
			default:
				stringArgs = append(stringArgs, fmt.Sprintf("%v", v))
			}
		}

		newProc.Args = append(newProc.Args, stringArgs...)

		result := newProc.Run()
		return result.Result(), nil
	}
}

func (p Process) Result() *ExecResult {

	r := &ExecResult{
		Stdout:   p.captureOutput.GetStdout(),
		Stderr:   p.captureOutput.GetStderr(),
		ExitCode: -1,
		Error:    p.Err,
		Status:   "pending",
		Started:  p.Started,
		Command:  p.Cmd,
	}

	if len(p.Args) > 0 {
		r.Command = fmt.Sprintf("%s %s", p.Cmd, strings.Join(p.Args, " "))
	}

	if p.cmd != nil && p.cmd.ProcessState != nil {
		if p.Err == nil {
			r.Status = "success"
		} else if strings.Contains(p.Err.Error(), "timed out") {
			r.Status = "timeout"
		} else if strings.Contains(p.Err.Error(), "permission denied") {
			r.Status = "permission denied"
		} else {
			r.Status = "failed"
		}
	}

	if p.cmd != nil && p.cmd.Process != nil {
		r.PID = p.cmd.Process.Pid
	}

	if p.Started != nil {
		r.Duration = time.Since(*p.Started)
	}
	return r
}

func (p Process) Out() string {
	return p.captureOutput.GetOutput()
}

func (p Process) Pretty() api.Text {
	return p.Result().Pretty()
}

func (p Process) WithEnv(env map[string]string) Process {
	p.Env = env
	return p
}

func (p Process) WithCwd(cwd string) Process {
	p.Cwd = cwd
	return p
}

func (p Process) WithTask(t *task.Task) Process {
	p.task = t
	return p
}

func (p Process) WithTimeout(timeout time.Duration) Process {
	p.Timeout = timeout
	return p
}

func (p Process) Name() string {
	if p.cmd != nil {
		return p.cmd.Path
	}
	return p.Cmd
}

// Start runs the process in the background
func (p Process) Start() error {
	shutdown.AddHook("Stopping "+p.Name(), func() {
		_ = p.MustStop(10 * time.Second) // ignore error during shutdown
	})
	go p.Run()
	return nil
}

// MustStop attempts to gracefully stop a process, after which it is forcefully killed
func (p Process) MustStop(timeout time.Duration) error {
	if err := p.Terminate(); err != nil {
		return err
	}
	return nil
}

func (p Process) Stop() error {
	if err := p.Terminate(); err != nil {
		return err
	}
	return nil
}

func (p Process) GetOutput() string {
	return p.captureOutput.GetOutput()
}

func (p Process) GetStdout() string {
	return p.captureOutput.GetStdout()
}

func (p Process) GetStderr() string {
	return p.captureOutput.GetStderr()
}

func (p Process) Debug() Process {
	if p.log == nil {
		p.log = logger.GetLogger()
	}
	p.log = p.log.WithV(logger.Debug)
	if p.captureOutput == nil {
		p.captureOutput = NewExecLogger()
	}
	p.captureOutput = p.captureOutput.Tee(os.Stdout, os.Stdin)
	return p
}

func (p Process) Short() api.Text {
	t := api.Text{Content: p.Cmd, Style: api.Clz(p.IsOK(), "text-green-500")}
	t.Append(strings.Join(p.Args, " "), "text-muted max-width-[10ch]")

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

// Runf runs the process and returns the result
func (p Process) Runf(sh string, args ...interface{}) Process {
	p.Cmd = fmt.Sprintf(sh, args...)
	return p.Run()
}

func (p Process) Run() Process {
	if p.log == nil {
		p.log = logger.GetLogger()
	}
	if p.captureOutput == nil {
		p.captureOutput = NewExecLogger()
	}

	if properties.On(true, "exec.log") {
		p.log = p.log.WithV(logger.Trace)
	}

	var cmd *exec.Cmd

	// If Args is empty, treat Cmd as a shell command
	if len(p.Args) == 0 {
		// Check if command contains shell operators
		if ContainsShellOperators(p.Cmd) {
			// Try bash first, fall back to sh
			shellBin := "bash"
			if _, err := exec.LookPath("bash"); err != nil {
				shellBin = "sh"
			}
			cmd = exec.Command(shellBin, "-c", p.Cmd)
		} else {
			// No shell operators, use sh
			cmd = exec.Command("/bin/sh", "-c", p.Cmd)
		}
	} else {
		cmd = exec.Command(p.Cmd, p.Args...)
	}

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

	p.log.Debugf(api.Text{Content: "Executing: "}.Add(p.Short()).ANSI())

	now := time.Now()
	p.Started = &now
	p.cmd = cmd

	// Handle timeout if configured
	if p.Timeout > 0 {
		resultChan := make(chan error, 1)
		go func() {
			resultChan <- cmd.Run()
		}()

		select {
		case err := <-resultChan:
			p.Err = err
		case <-time.After(p.Timeout):
			_ = p.Kill(5 * time.Second)
			p.Err = fmt.Errorf("command timed out after %v", time.Since(*p.Started))
		}
	} else {
		p.Err = cmd.Run()
	}

	switch v := p.Err.(type) {
	case *exec.ExitError:
		p.exitCode = lo.ToPtr(v.ExitCode())
		// nil out error if non-zero exit codes are not considered errors
		if p.ErrorOnNonZero {
			p.Err = fmt.Errorf("exit code: %d", *p.exitCode)
		} else {
			// non-zero exit codes are not considered errors
			p.Err = nil
		}
		// Enhance fork/exec permission errors with better messages
		if v.ExitCode() == 126 {
			p.Err = fmt.Errorf("permission denied: %s", p.Cmd)
		}
	}
	if p.Err != nil {
		p.log.Warnf(p.Pretty().ANSI())
	} else {
		p.log.Tracef(p.Pretty().ANSI())
	}

	return p
}

func (p Process) ExitCode() int {
	if p.cmd != nil && p.cmd.ProcessState != nil {
		return p.cmd.ProcessState.ExitCode()
	}
	return -1
}

func (p Process) IsRunning() bool {
	return p.cmd != nil && p.cmd.Process != nil && !p.cmd.ProcessState.Exited()
}

func (p Process) IsOK() bool {
	return p.Err == nil && p.cmd != nil && p.cmd.ProcessState != nil && p.cmd.ProcessState.Success()
}

func (p Process) Wait() error {
	if p.cmd == nil {
		return nil
	}
	return p.cmd.Wait()
}

func (p Process) Terminate() error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
		return err
	}
	_, err := p.cmd.Process.Wait()
	return err
}

func (p Process) Kill(timeout time.Duration) error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		_, err := p.cmd.Process.Wait()
		done <- err
	}()

	select {
	case <-time.After(timeout):
		return p.ForceKill()
	case err := <-done:
		return err
	}
}

func (p Process) ForceKill() error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

// WaitForStdout waits for a specific message to appear in the process stdout
// This is useful for waiting for server startup messages like "Server started on port"
func (p *Process) WaitForStdout(message string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Check if process has exited with error
		if p.cmd != nil && p.cmd.ProcessState != nil && !p.cmd.ProcessState.Success() {
			return fmt.Errorf("process exited with error: %v", p.Err)
		}

		// Check stdout buffer for the message
		stdout := p.GetStdout()
		if strings.Contains(stdout, message) {
			return nil
		}

		// Brief sleep to avoid busy waiting
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for message '%s' in stdout after %v", message, timeout)
}

// GetTask implements the Taskable interface
func (p *Process) GetTask() *task.Task {
	return p.task
}

// StartAsTask creates and starts a Task for this Process with typed result handling
func (p *Process) RunAsTask(name string, opts ...task.Option) task.TypedTask[ExecResult] {
	taskFunc := func(ctx cctx.Context, t *task.Task) (ExecResult, error) {
		p.task = t
		ginkgo.GinkgoWriter.Write([]byte("before:" + p.Pretty().ANSI() + "\n"))
		out := p.Run()
		*p = out
		ginkgo.GinkgoWriter.Write([]byte("oput:" + out.Pretty().ANSI() + "\n"))
		ginkgo.GinkgoWriter.Write([]byte("p:" + p.Pretty().ANSI() + "\n"))

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
		*p = p.Run()

		err := p.Err
		// Return the result and error
		return p, err
	}

	return task.StartTask(name, taskFunc, opts...)
}
