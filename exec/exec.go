package exec

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/shutdown"
	"github.com/flanksource/clicky/task"
	"github.com/flanksource/commons/context"
	"github.com/flanksource/commons/logger"
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
	Started *time.Time
	cmd     *exec.Cmd
	task    *task.Task
	Env     map[string]string
	Cwd     string
	Err     error
	Log     logger.Logger
	Stderr  bytes.Buffer
	Stdout  bytes.Buffer
	Cmd     string
	Args    []string
}

func (p Process) Out() string {
	return p.Stderr.String() + p.Stdout.String()
}

func (p Process) Pretty() api.Text {
	return api.Text{Content: p.Name()}
}

func (p Process) WithEnv(env map[string]string) Process {
	p.Env = env
	return p
}

func (p Process) WithCwd(cwd string) Process {
	p.Cwd = cwd
	return p
}

func (p Process) WithLogger(log logger.Logger) Process {
	p.Log = log
	return p
}

func (p Process) WithTask(t *task.Task) Process {
	p.task = t
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

func (p Process) Kill() error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

// Runf runs the process and returns the result
func (p Process) Runf(sh string, args ...interface{}) Process {
	p.Cmd = fmt.Sprintf(sh, args...)
	return p.Run()
}

func (p Process) Run() Process {
	var cmd *exec.Cmd

	// If Args is empty, treat Cmd as a shell command
	if len(p.Args) == 0 {
		// Check if command contains shell operators
		if ContainsShellOperators(p.Cmd) {
			// Try bash first, fall back to sh
			shellBin := "bash"
			if _, err := exec.LookPath("bash"); err != nil {
				shellBin = "sh"
				if p.task != nil {
					p.task.V(5).Infof("bash not found, using sh instead")
				}
			} else if p.task != nil {
				p.task.V(5).Infof("Using bash for shell command")
			}
			cmd = exec.Command(shellBin, "-c", p.Cmd)
		} else {
			// No shell operators, use sh
			cmd = exec.Command("/bin/sh", "-c", p.Cmd)
		}
	} else {
		cmd = exec.Command(p.Cmd, p.Args...)
	}

	cmd.Dir = p.Cwd
	if p.Cwd != "" && p.task != nil {
		p.task.V(4).Infof("Setting working directory to %s", p.Cwd)
	}

	cmd.Stderr = &p.Stderr
	cmd.Stdout = &p.Stdout

	// Set environment variables
	cmd.Env = os.Environ() // Start with current environment
	for k, v := range p.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	now := time.Now()
	p.Started = &now
	p.cmd = cmd
	p.Err = cmd.Run()

	return p
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
		stdout := p.Stdout.String()
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

// AsTask converts the Process into a Task that can be managed by TaskManager
func (p *Process) AsTask(name string, opts ...task.Option) *task.Task {
	taskFunc := func(ctx context.Context, t *task.Task) (*Process, error) {
		// Store the task reference immediately
		p.task = t

		// Run the process
		result := p.Run()

		// Log the output if there's any
		if output := result.Out(); output != "" {
			t.Infof("Process output: %s", output)
		}

		// Update the original process with results while preserving task reference
		p.Started = result.Started
		p.cmd = result.cmd
		p.Env = result.Env
		p.Cwd = result.Cwd
		p.Err = result.Err
		p.Log = result.Log
		p.Stderr = result.Stderr
		p.Stdout = result.Stdout
		p.Cmd = result.Cmd
		p.Args = result.Args
		// Keep p.task as it is

		// Return the result and error
		return p, result.Err
	}

	// Use StartTask and return the underlying Task
	typedTask := task.StartTask(name, taskFunc, opts...)
	return typedTask.GetTask()
}

// StartAsTask creates and starts a Task for this Process with typed result handling
func (p *Process) StartAsTask(name string, opts ...task.Option) task.TypedTask[Process] {
	taskFunc := func(ctx context.Context, t *task.Task) (Process, error) {
		// Store the task reference
		p.task = t

		// Run the process
		result := p.Run()

		// Log the output if there's any
		if output := result.Out(); output != "" {
			t.Infof("Process output: %s", output)
		}

		// Update the original process with results while preserving task reference
		p.Started = result.Started
		p.cmd = result.cmd
		p.Env = result.Env
		p.Cwd = result.Cwd
		p.Err = result.Err
		p.Log = result.Log
		p.Stderr = result.Stderr
		p.Stdout = result.Stdout
		p.Cmd = result.Cmd
		p.Args = result.Args
		// Keep p.task as it is

		// Return the result and error
		return result, result.Err
	}

	return task.StartTask(name, taskFunc, opts...)
}
