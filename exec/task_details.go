package exec

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
)

// ExecTaskDetails is the structured command metadata exposed beside stdout and
// stderr in task snapshots.
type ExecTaskDetails struct {
	Command  string        `json:"command"`
	Args     []string      `json:"args,omitempty"`
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
	if p.captureOutput == nil {
		p.captureOutput = NewExecLogger()
	}
	p.task = t
	t.SetOutputProvider(func() task.OutputSnapshot {
		return task.OutputSnapshot{Stdout: p.GetStdout(), Stderr: p.GetStderr()}
	})
	t.SetDetailsProvider(func() any {
		result := p.Result()
		return ExecTaskDetails{
			Command:  result.Command,
			Args:     append([]string(nil), result.Args...),
			PID:      result.PID,
			Status:   result.Status,
			ExitCode: result.ExitCode,
			Started:  result.Started,
			Duration: result.Duration,
		}
	})
	t.SetController(&processTaskController{process: p, task: t})

	done := make(chan struct{})
	go killProcessOnCancellation(ctx, p, done)
	return func() { close(done) }
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
