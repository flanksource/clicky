package exec

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/icons"
	"github.com/flanksource/clicky/task"
	cctx "github.com/flanksource/commons/context"
)

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
	return p.running && p.pid != 0
}

func (p *Process) IsOK() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Err == nil && p.completed && p.succeeded
}

// Wait blocks until the in-flight Run() has published its terminal state and
// returns the process error. Run() is the only reaper of the child (the sole
// cmd.Wait caller); Wait merely observes its published result via the done
// channel, so it can never reap the child out from under Run().
func (p *Process) Wait() error {
	p.mu.RLock()
	done := p.done
	p.mu.RUnlock()
	if done != nil {
		<-done
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Err
}

func (p *Process) Terminate() error {
	if !p.IsRunning() {
		return nil
	}
	p.log.Infof("%s", api.Text{}.Add(icons.Stop).Space().Append("Terminating process: ").Add(p.PrettyShort()).ANSI())
	p.mu.RLock()
	proc := p.cmd.Process
	p.mu.RUnlock()
	if err := proc.Signal(os.Interrupt); err != nil {
		p.log.Errorf("%s", api.Text{}.Add(icons.Fail).Space().Append(" failed to signal ").Add(p.PrettyShort()).Append(err, "text-red-500").ANSI())
		return err
	}
	if p.done != nil {
		select {
		case <-p.done:
			p.log.Tracef("%s", api.Text{}.Append("terminated ").Add(p.PrettyShort()).ANSI())
		case <-time.After(5 * time.Second):
			p.log.Warnf("Timeout waiting for Run() to complete after termination")
			return fmt.Errorf("timeout waiting for process termination")
		}
	}
	return nil
}

// Kill interrupts the process and waits for Run() — the only reaper — to
// publish its terminal state (escalating to SIGKILL after timeout). It never
// waits on the child itself, so it cannot reap it out from under Run().
func (p *Process) Kill(timeout time.Duration) error {
	if !p.IsRunning() {
		return nil
	}
	p.log.Infof("%s", api.Text{}.Add(icons.Zombie).Space().Append("Killing process: ").Add(p.PrettyShort()).Append(fmt.Sprintf(" with timeout %v", timeout), "text-muted").ANSI())
	p.mu.RLock()
	proc := p.cmd.Process
	done := p.done
	p.mu.RUnlock()
	if err := proc.Signal(os.Interrupt); err != nil {
		return err
	}
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
	}
	if err := p.ForceKill(); err != nil {
		return err
	}
	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for process to exit after kill")
	}
}

// stopAndReap interrupts the child and waits for Run()'s single cmd.Wait
// goroutine to reap it via resultChan, escalating to SIGKILL after grace. It
// never calls cmd.Wait/Process.Wait itself, keeping Run() the only reaper.
func (p *Process) stopAndReap(resultChan <-chan error, grace time.Duration) {
	p.mu.RLock()
	var proc *os.Process
	if p.cmd != nil {
		proc = p.cmd.Process
	}
	p.mu.RUnlock()
	if proc != nil {
		_ = proc.Signal(os.Interrupt)
	}
	select {
	case <-resultChan:
	case <-time.After(grace):
		_ = p.ForceKill()
		<-resultChan
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

func (p *Process) WaitForStdout(message string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		p.mu.RLock()
		failed := p.completed && !p.succeeded
		procErr := p.Err
		p.mu.RUnlock()
		if failed {
			return fmt.Errorf("process exited with error: %v", procErr)
		}
		if strings.Contains(p.GetStdout(), message) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting '%s' in stdout after %v", message, timeout)
}

func (p *Process) GetTask() *task.Task { return p.task }

func (p *Process) RunAsTask(name string, opts ...task.Option) task.TypedTask[ExecResult] {
	taskFunc := func(ctx cctx.Context, task *task.Task) (ExecResult, error) {
		done := bindProcessTask(ctx, task, p)
		defer done()
		p = p.Run()
		return *p.Result(), p.Err
	}
	return task.StartTask(name, taskFunc, opts...)
}

func (p *Process) StartAsTask(name string, opts ...task.Option) task.TypedTask[*Process] {
	taskFunc := func(ctx cctx.Context, task *task.Task) (*Process, error) {
		done := bindProcessTask(ctx, task, p)
		defer done()
		p = p.Run()
		return p, p.Err
	}
	return task.StartTask(name, taskFunc, opts...)
}
