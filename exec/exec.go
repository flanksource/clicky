package exec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/shutdown"
	"github.com/flanksource/commons/logger"
)

type Process struct {
	Started *time.Time
	cmd     *exec.Cmd
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

func (p Process) Name() string {
	return p.cmd.Path
}

// Start runs the process in the background
func (p Process) Start() error {
	shutdown.AddHook("Stopping "+p.Name(), func() {
		p.MustStop(10 * time.Second)
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
	return p.cmd.Process.Kill()
}

// Runf runs the process and returns the result
func (p Process) Runf(sh string, args ...interface{}) Process {
	p.Cmd = fmt.Sprintf(sh, args...)
	return p.Run()
}

func (p Process) Run() Process {
	cmd := exec.Command("bash", "-c", p.Cmd)
	cmd.Dir = p.Cwd
	cmd.Stderr = io.MultiWriter(&p.Stderr, os.Stderr)
	cmd.Stdout = io.MultiWriter(&p.Stdout, os.Stdout)
	cmd.Stdin = os.Stdin

	for k, v := range p.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	p.Err = cmd.Run()

	return p
}

func (p Process) IsOK() bool {
	return p.Err == nil && p.cmd.ProcessState != nil && p.cmd.ProcessState.Success()
}

func (p Process) Wait() error {
	return p.cmd.Wait()
}

func (p Process) Terminate() error {
	if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
		return err
	}
	_, err := p.cmd.Process.Wait()
	return err
}

func (p Process) ForceKill() error {
	return p.cmd.Process.Kill()
}
