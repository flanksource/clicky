package exec

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/flanksource/commons/logger"
)

// WrapperFunc is a function type returned by AsWrapper that executes commands
// with pre-configured settings from a template Process.
type WrapperFunc func(...any) (*ExecResult, error)

// WrapperOption is a functional option that modifies a Process for a single execution.
type WrapperOption interface {
	apply(*Process)
}

type wrapperOptionFunc func(*Process)

func (f wrapperOptionFunc) apply(p *Process) { f(p) }

// WithTimeout returns a WrapperOption that overrides the execution timeout.
func WithTimeout(timeout time.Duration) WrapperOption {
	return wrapperOptionFunc(func(p *Process) { p.Timeout = timeout })
}

// WithContext returns a WrapperOption that cancels the execution when ctx is
// done: an already-canceled context (or expired deadline) prevents the command
// from starting, and cancellation during execution terminates the process
// while Run() reaps it exactly once.
func WithContext(ctx context.Context) WrapperOption {
	return wrapperOptionFunc(func(p *Process) { p.ctx = ctx })
}

func WithTee(stdout, stderr io.Writer) WrapperOption {
	return wrapperOptionFunc(func(p *Process) {
		if p.captureOutput == nil {
			p.captureOutput = NewExecLogger()
		}
		p.captureOutput = p.captureOutput.Tee(stdout, stderr)
	})
}

func WithLogger(log logger.Logger) WrapperOption {
	return wrapperOptionFunc(func(p *Process) { p.log = log })
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
	return wrapperOptionFunc(func(p *Process) { p.Cwd = path })
}

// WithoutErrorOnNonZero returns a WrapperOption that makes non-zero exit codes succeed.
func WithoutErrorOnNonZero() WrapperOption {
	return wrapperOptionFunc(func(p *Process) { p.SucceedOnNonZero = true })
}

func WithDebug() WrapperOption {
	return wrapperOptionFunc(func(p *Process) { p.Debug() })
}

// WithStdioPipe connects the child's stdin to a writable pipe (accessible via
// Stdin) and its stdout to a readable pipe (accessible via StdoutReader),
// enabling bidirectional line-protocol communication such as JSON-RPC.
func (p *Process) WithStdioPipe() *Process {
	p.wantStdioPipe = true
	return p
}

func (p *Process) Stdin() io.WriteCloser {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stdin
}

func (p *Process) StdoutReader() io.Reader {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stdoutR
}

func (p *Process) clone() *Process {
	var capture *ExecLogger
	if p.captureOutput != nil {
		capture = NewExecLogger().Tee(p.captureOutput.Stdout, p.captureOutput.Stderr)
	}
	cloned := &Process{
		Cwd:              p.Cwd,
		Cmd:              p.Cmd,
		Args:             make([]string, len(p.Args)),
		Timeout:          p.Timeout,
		ctx:              p.ctx,
		captureOutput:    capture,
		SucceedOnNonZero: p.SucceedOnNonZero,
		log:              p.log,
		task:             p.task,
		Shell:            p.Shell,
		exactEnvironment: p.exactEnvironment,
		newProcessGroup:  p.newProcessGroup,
		wantStdioPipe:    p.wantStdioPipe,
		input:            p.input,
		done:             make(chan struct{}),
	}
	for key, value := range p.Env {
		if cloned.Env == nil {
			cloned.Env = make(map[string]string, len(p.Env))
		}
		cloned.Env[key] = value
	}
	copy(cloned.Args, p.Args)
	return cloned
}

func (p *Process) AsWrapper() WrapperFunc {
	return func(args ...any) (*ExecResult, error) {
		var stringArgs []string
		newProc := p.clone()
		for _, arg := range args {
			switch value := arg.(type) {
			case WrapperOption:
				value.apply(newProc)
			case string:
				stringArgs = append(stringArgs, value)
			default:
				stringArgs = append(stringArgs, fmt.Sprintf("%v", value))
			}
		}
		newProc.Args = append(newProc.Args, stringArgs...)
		result := newProc.Run()
		res := result.Result()
		if result.Err != nil {
			return res, result.Err
		}
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
