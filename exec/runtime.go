package exec

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	gops "github.com/shirou/gopsutil/v4/process"
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
	binary, args, err = p.runtimeCommand(binary, args, 0)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(binary, args...)
	cmd.Dir, cmd.Env, cmd.Stdin = p.Cwd, p.environment(), p.input
	cmd.ExtraFiles = p.extraFiles
	applyProcessGroup(cmd, p.newProcessGroup)
	return cmd, nil
}

// usesLauncher reports whether the target runs behind the niceness launcher.
func (p *Process) usesLauncher() bool {
	return p.niceness != nil && *p.niceness != 0
}

// runtimeCommand wraps the target in the niceness launcher when one is needed.
// A statusFD above 2 is the child descriptor the launcher writes to when it
// fails before executing the target; 0 leaves the exit status 125 as the only
// signal.
func (p *Process) runtimeCommand(binary string, args []string, statusFD int) (string, []string, error) {
	l := p.resourceLimits
	if math.IsNaN(l.MaxCPUPercent) || math.IsInf(l.MaxCPUPercent, 0) || l.MaxCPUPercent < 0 {
		return "", nil, fmt.Errorf("invalid CPU limit: %v", l.MaxCPUPercent)
	}
	if !p.usesLauncher() {
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
	launcher := `"$1" "$2" -p "$$" >/dev/null || exit 125
shift 2
exec "$@"`
	if statusFD > 2 {
		// The status descriptor is closed for renice and before exec, so only a
		// launcher failure ever writes to it and the target never inherits it.
		launcher = fmt.Sprintf(`"$1" "$2" -p "$$" >/dev/null %[1]d>&- || { echo "renice exited $?" >&%[1]d; exit 125; }
exec %[1]d>&-
shift 2
exec "$@"`, statusFD)
	}
	return shell, append([]string{"-c", launcher, "clicky-niceness", renice, strconv.Itoa(priority), binary}, args...), nil
}

// launchStatus reports whether the niceness launcher failed before it executed
// the target, independently of the target's own exit status.
type launchStatus struct {
	reader, writer *os.File
}

func newLaunchStatus() (*launchStatus, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("niceness status pipe: %w", err)
	}
	return &launchStatus{reader: reader, writer: writer}, nil
}

// started closes the parent's write end once the child holds its own copy.
func (s *launchStatus) started() {
	_ = s.writer.Close()
}

// err reads what the launcher reported. It must run after the launcher exited
// or executed the target, which leaves no open write end and makes the read
// return immediately; the deadline guards a launcher that was never reaped.
func (s *launchStatus) err() error {
	defer func() { _ = s.reader.Close() }()
	_ = s.reader.SetReadDeadline(time.Now().Add(time.Second))
	report, err := io.ReadAll(s.reader)
	if len(report) > 0 {
		return fmt.Errorf("niceness: launcher failed to set child priority (%s); the command was not executed",
			strings.TrimSpace(string(report)))
	}
	if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
		return fmt.Errorf("niceness: read launcher status: %w", err)
	}
	return nil
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
