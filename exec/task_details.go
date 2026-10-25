package exec

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
)

// ExecTaskDetails is the structured command metadata exposed beside stdout and
// stderr in task snapshots.
type ExecTaskDetails struct {
	Command  string        `json:"command"`
	Args     []string      `json:"args,omitempty"`
	Cwd      string        `json:"cwd,omitempty"`
	PID      int           `json:"pid,omitempty"`
	Status   string        `json:"status"`
	ExitCode int           `json:"exitCode"`
	Started  *time.Time    `json:"started,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`
}

type processTaskController struct {
	process *Process
	task    *task.Task
}

func (c *processTaskController) Actions() []task.ControlAction {
	status := c.task.Status()
	if status == task.StatusPending || status == task.StatusRunning {
		return []task.ControlAction{task.ControlStop}
	}
	return nil
}

func (c *processTaskController) Control(_ context.Context, action task.ControlAction) error {
	if action != task.ControlStop {
		return fmt.Errorf("exec task does not support %q", action)
	}
	c.task.Cancel()
	return nil
}

func (c *processTaskController) WriteStdin(data []byte) error {
	stdin := c.process.Stdin()
	if stdin == nil {
		return fmt.Errorf("exec task stdin is not available")
	}
	_, err := stdin.Write(data)
	return err
}

func bindProcessTask(ctx flanksourceContext.Context, t *task.Task, p *Process) func() {
	p.WithTask(t)
	done := make(chan struct{})
	go killProcessOnCancellation(ctx, p, done)
	return func() { close(done) }
}

func (p *Process) bindTask(t *task.Task) {
	if p.captureOutput == nil {
		p.captureOutput = NewExecLogger()
	}
	p.task = t
	t.SetOutputProvider(func() task.OutputSnapshot {
		return task.OutputSnapshot{Stdout: p.GetStdout(), Stderr: p.GetStderr()}
	})
	t.SetDetailsProvider(func() any { return p.taskDetails() })
	t.SetController(&processTaskController{process: p, task: t})
}

func (p *Process) taskDetails() ExecTaskDetails {
	p.mu.RLock()
	defer p.mu.RUnlock()
	details := ExecTaskDetails{Command: p.Cmd, Args: append([]string(nil), p.Args...), Cwd: p.Cwd, PID: p.pid, Started: p.Started, ExitCode: -1, Status: "pending"}
	if p.exitCode != nil {
		details.ExitCode = *p.exitCode
	}
	switch {
	case p.Err != nil && strings.Contains(p.Err.Error(), "timed out"):
		details.Status = "timeout"
	case p.Err != nil || p.completed && details.ExitCode != 0:
		details.Status = "failed"
	case p.completed:
		details.Status = "success"
	case p.running:
		details.Status = "running"
	}
	details.Duration = p.durationLocked()
	return details
}

func killProcessOnCancellation(ctx context.Context, p *Process, done <-chan struct{}) {
	select {
	case <-ctx.Done():
	case <-done:
		return
	}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if p.Pid() > 0 {
			_ = p.KillTree()
			return
		}
		select {
		case <-done:
			return
		case <-deadline.C:
			return
		case <-ticker.C:
		}
	}
}

type capturingReadCloser struct {
	io.Reader
	io.Closer
}
