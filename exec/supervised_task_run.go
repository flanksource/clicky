package exec

import (
	"fmt"

	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
)

type RunSupervisedTaskOptions struct {
	Name      string
	Supervise SuperviseOptions
	Task      []task.Option
}

func (p *Process) RunSupervisedAsTask(options RunSupervisedTaskOptions) task.TypedTask[ExecResult] {
	name := options.Name
	if name == "" {
		name = p.Name()
	}
	return task.StartTask(name, func(ctx flanksourceContext.Context, current *task.Task) (ExecResult, error) {
		if policy := options.Supervise.RestartPolicy; policy != "" && policy != RestartNo {
			return ExecResult{}, fmt.Errorf("supervised task restart policy must be %q", RestartNo)
		}
		supervisor := p.Supervise(options.Supervise)
		supervisor.mu.Lock()
		supervisor.boundTask = current
		supervisor.mu.Unlock()
		// An already-cancelled task never starts the process at all.
		if err := ctx.Err(); err != nil {
			return ExecResult{Status: "cancelled", ExitCode: -1}, err
		}

		// Start before the cancellation watcher is installed. Stop() is a no-op
		// while the supervise loop is inactive, so a watcher that fired in the
		// gap before Start() would leave the process running with nothing left
		// to cancel it.
		supervisor.Start()

		cancelled := make(chan struct{})
		defer close(cancelled)
		go func() {
			select {
			case <-ctx.Done():
				supervisor.Stop()
			case <-cancelled:
			}
		}()
		supervisor.Wait()
		result := supervisor.Result()
		return result, result.Error
	}, options.Task...)
}
