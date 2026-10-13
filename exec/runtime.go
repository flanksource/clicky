package exec

import (
	"fmt"
	gops "github.com/shirou/gopsutil/v4/process"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"
)

type ResourceLimitError struct {
	Reason string
}

func (e *ResourceLimitError) Error() string {
	return fmt.Sprintf("resource limit exceeded: %s", e.Reason)
}

func (p *Process) WithResourceLimits(limits ResourceLimits) *Process {
	p.resourceLimits = limits
	return p
}

func (p *Process) WithNiceness(nice int) *Process {
	p.niceness = &nice
	return p
}

func (p *Process) WithExtraFiles(files []*os.File) *Process {
	p.extraFiles = append([]*os.File(nil), files...)
	return p
}

func (p *Process) ResourceLimits() ResourceLimits { return p.resourceLimits }

func (p *Process) PrepareCommand() (*exec.Cmd, error) {
	binary, args, err := p.parseCommand()
	if err != nil {
		return nil, err
	}
	binary, args, err = p.runtimeCommand(binary, args)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(binary, args...)
	cmd.Dir, cmd.Env, cmd.Stdin = p.Cwd, p.environment(), p.input
	cmd.ExtraFiles = p.extraFiles
	applyProcessGroup(cmd, p.newProcessGroup)
	return cmd, nil
}

func (p *Process) runtimeCommand(binary string, args []string) (string, []string, error) {
	l := p.resourceLimits
	if math.IsNaN(l.MaxCPUPercent) || math.IsInf(l.MaxCPUPercent, 0) || l.MaxCPUPercent < 0 {
		return "", nil, fmt.Errorf("invalid CPU limit: %v", l.MaxCPUPercent)
	}
	if p.niceness == nil || *p.niceness == 0 {
		return binary, args, nil
	}
	if *p.niceness < -20 || *p.niceness > 19 || runtime.GOOS == "windows" {
		return "", nil, fmt.Errorf("unsupported niceness %d on %s", *p.niceness, runtime.GOOS)
	}
	parent, err := gops.NewProcess(int32(os.Getpid()))
	if err != nil {
		return "", nil, fmt.Errorf("niceness: %w", err)
	}
	current, err := parent.Nice()
	if err != nil {
		return "", nil, fmt.Errorf("read parent niceness: %w", err)
	}
	priority := max(-20, min(19, int(current)+*p.niceness))
	renice, err := exec.LookPath("renice")
	if err != nil {
		return "", nil, fmt.Errorf("niceness: %w", err)
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		return "", nil, fmt.Errorf("niceness: %w", err)
	}
	// nice can execute the target after failing to change priority. Refuse that execution.
	const launcher = `"$1" "$2" -p "$$" >/dev/null
status=$?
if [ "$status" -ne 0 ]
then
  exit 125
fi
shift 2
exec "$@"`
	return shell, append([]string{"-c", launcher, "clicky-niceness", renice, strconv.Itoa(priority), binary}, args...), nil
}

func (p *Process) waitRuntime(result <-chan error, timeout <-chan time.Time, cancelled <-chan struct{}) error {
	limits := p.resourceLimits
	if limits.Interval <= 0 {
		limits.Interval = defaultSampleInterval
	}
	if limits.CPUSampleCount <= 0 {
		limits.CPUSampleCount = defaultCPUSampleCount
	}
	monitor := &SupervisedProcess{current: p, template: p, opts: SuperviseOptions{Limits: limits}, handles: make(map[int32]*gops.Process)}
	var samples <-chan time.Time
	if limits.enabled() {
		ticker := time.NewTicker(limits.Interval)
		defer ticker.Stop()
		samples = ticker.C
		monitor.sample()
	}
	for {
		select {
		case err := <-result:
			if monitor.limitError != nil {
				return monitor.limitError
			}
			return err
		case <-samples:
			monitor.sample()
		case <-timeout:
			p.stopAndReap(result, 5*time.Second)
			return fmt.Errorf("command timed out after %v", p.Timeout)
		case <-cancelled:
			p.stopAndReap(result, 5*time.Second)
			return p.ctx.Err()
		}
	}
}
