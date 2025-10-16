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
	Timeout time.Duration
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
	// Build command string
	cmdStr := p.Cmd
	if len(p.Args) > 0 {
		cmdStr = fmt.Sprintf("%s %s", p.Cmd, strings.Join(p.Args, " "))
	}

	// Determine status
	status := "pending"
	exitCode := -1

	if p.cmd != nil && p.cmd.ProcessState != nil {
		if exitErr, ok := p.Err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if p.Err == nil {
			exitCode = 0
		}

		if p.Err == nil {
			status = "success"
		} else if strings.Contains(p.Err.Error(), "timed out") {
			status = "timeout"
		} else if strings.Contains(p.Err.Error(), "permission denied") {
			status = "permission denied"
		} else {
			status = "failed"
		}
	}

	// Get first line of output if available
	output := p.Out()
	firstLine := ""
	if output != "" {
		lines := strings.Split(output, "\n")
		if len(lines) > 0 {
			firstLine = TruncateString(lines[0], 100)
		}
	}

	// Build pretty output
	content := fmt.Sprintf("Command: %s\nStatus: %s", cmdStr, status)
	if exitCode >= 0 {
		content = fmt.Sprintf("%s (exit: %d)", content, exitCode)
	}
	if firstLine != "" {
		content = fmt.Sprintf("%s\nOutput: %s", content, firstLine)
	}
	if p.Err != nil {
		content = fmt.Sprintf("%s\nError: %v", content, p.Err)
	}

	return api.Text{Content: content}
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

	// Log command before execution if task logger is available
	if p.task != nil {
		cmdStr := p.Cmd
		if len(p.Args) > 0 {
			cmdStr = fmt.Sprintf("%s %s", p.Cmd, strings.Join(p.Args, " "))
		}

		// V(3): Log full command
		p.task.V(3).Infof("Executing: %s", cmdStr)

		// Debug: Log executing message
		if p.task.IsDebugEnabled() {
			p.task.Debugf("Executing: %s", cmdStr)
		}
	}

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
			// Try to kill the running process
			_ = p.cmd.Process.Kill()
			p.Err = fmt.Errorf("command timed out after %v", p.Timeout)
			if p.task != nil {
				p.task.V(4).Infof("Command timed out after %v", p.Timeout)
			}
		}
	} else {
		p.Err = cmd.Run()
	}

	// Enhance fork/exec permission errors with better messages
	if p.Err != nil && strings.Contains(p.Err.Error(), "fork/exec") && strings.Contains(p.Err.Error(), "permission denied") {
		// Check if the command file exists and get permissions
		checkPath := p.Cmd
		if info, statErr := os.Stat(checkPath); statErr == nil {
			p.Err = fmt.Errorf("binary %s exists but is not executable (permissions: %s). Try: chmod +x %s",
				checkPath, info.Mode().String(), checkPath)
			if p.task != nil {
				p.task.V(4).Infof("Permission error: %v", p.Err)
			}
		}
	}

	// Log output after execution if task logger is available
	if p.task != nil {
		cmdStr := p.Cmd
		if len(p.Args) > 0 {
			cmdStr = fmt.Sprintf("%s %s", p.Cmd, strings.Join(p.Args, " "))
		}

		// Get exit code
		exitCode := 0
		if p.Err != nil {
			if exitErr, ok := p.Err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}

		// Log output based on verbosity level
		output := p.Out()

		// V(3): Log complete output
		if p.task.IsLevelEnabled(logger.Trace2) {
			p.task.V(3).Infof("Command: %s\nExit code: %d\nOutput:\n%s", cmdStr, exitCode, output)
		} else if p.task.IsLevelEnabled(logger.Trace) {
			// Trace: up to 10 lines OR 1000 chars
			truncated := TruncateOutput(output, 10, 1000)
			p.task.Tracef("Command: %s\nExit code: %d\nOutput:\n%s", cmdStr, exitCode, truncated)
		} else if p.task.IsDebugEnabled() {
			// Debug: full output
			p.task.Debugf("Output:\n%s", output)
		} else if p.task.IsLevelEnabled(logger.Info) {
			// Info: first line truncated to 100 chars
			firstLine := GetFirstLine(output)
			truncated := TruncateString(firstLine, 100)
			p.task.Infof("$ %s (exit: %d) %s", cmdStr, exitCode, truncated)
		}
	}

	return p
}

// TruncateOutput truncates output to maxLines or maxChars, whichever limit is hit first
func TruncateOutput(output string, maxLines int, maxChars int) string {
	if len(output) == 0 {
		return output
	}

	lines := strings.Split(output, "\n")

	// Check line limit
	if len(lines) > maxLines {
		truncated := strings.Join(lines[:maxLines], "\n")
		remaining := len(lines) - maxLines
		return fmt.Sprintf("%s\n... (%d more lines)", truncated, remaining)
	}

	// Check char limit
	if len(output) > maxChars {
		truncated := output[:maxChars]
		remaining := len(output) - maxChars
		return fmt.Sprintf("%s... (truncated %d chars)", truncated, remaining)
	}

	return output
}

// GetFirstLine extracts the first line from output
func GetFirstLine(output string) string {
	if output == "" {
		return ""
	}
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		return lines[0]
	}
	return output
}

// TruncateString truncates a string to maxLen characters
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
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
