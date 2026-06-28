package exec

import (
	"sync"
	"time"

	gops "github.com/shirou/gopsutil/v3/process"
)

// Resource-monitor defaults applied when a ResourceLimits field is left zero.
const (
	defaultSampleInterval = 2 * time.Second
	defaultCPUSampleCount = 18 // ~36s over-limit at the 2s default before a CPU kill
)

// ResourceSnapshot is a point-in-time measurement of a supervised process and
// its descendant tree. OpenFiles is -1 on platforms that cannot report it.
type ResourceSnapshot struct {
	CPUPercent float64   `json:"cpuPercent"`
	RSSBytes   uint64    `json:"rssBytes"`
	OpenFiles  int       `json:"openFiles"`
	SampledAt  time.Time `json:"sampledAt"`
}

// ProcessSample is the resource usage of one process in the supervised group,
// the per-process breakdown behind the aggregate ResourceSnapshot. OpenFiles is
// -1 where the platform cannot report it.
type ProcessSample struct {
	PID        int32   `json:"pid"`
	PPID       int32   `json:"ppid"`
	Command    string  `json:"command"`
	CPUPercent float64 `json:"cpuPercent"`
	RSSBytes   uint64  `json:"rssBytes"`
	OpenFiles  int     `json:"openFiles"`
}

// ResourceLimits optionally bounds a supervised process. A zero value in any
// limit field disables that limit. When a limit is exceeded the whole process
// tree is killed (KillTree) and SupervisedProcess.Killed reports true.
type ResourceLimits struct {
	// MaxRSSBytes kills the process the first sample its tree RSS exceeds this.
	MaxRSSBytes uint64
	// MaxCPUPercent kills the process after CPUSampleCount consecutive samples
	// above this aggregate CPU percentage (transient spikes are tolerated).
	MaxCPUPercent float64
	// CPUSampleCount is the consecutive over-limit CPU samples tolerated before
	// a kill. Defaults to defaultCPUSampleCount.
	CPUSampleCount int
	// Interval is the sampling cadence. Defaults to defaultSampleInterval.
	Interval time.Duration
}

// enabled reports whether any runaway limit is configured.
func (l ResourceLimits) enabled() bool {
	return l.MaxRSSBytes > 0 || l.MaxCPUPercent > 0
}

// RestartPolicy controls whether a supervised process is restarted after it exits.
type RestartPolicy string

const (
	RestartNo        RestartPolicy = "no"         // never restart
	RestartOnFailure RestartPolicy = "on-failure" // restart only on a non-zero exit
	RestartAlways    RestartPolicy = "always"     // restart on any exit
)

// Status is the lifecycle state of a supervised process.
type Status string

const (
	StatusStopped    Status = "stopped"
	StatusStarting   Status = "starting"
	StatusCompiling  Status = "compiling"
	StatusRunning    Status = "running"
	StatusRestarting Status = "restarting"
	StatusCrashed    Status = "crashed"
	StatusExited     Status = "exited"
)

// SuperviseOptions configures the supervision loop.
type SuperviseOptions struct {
	// Limits bounds resource usage; zero values disable individual limits.
	Limits ResourceLimits
	// RestartPolicy controls restart-on-exit; defaults to RestartNo.
	RestartPolicy RestartPolicy
	// MaxRestarts caps automatic restarts (0 = unlimited).
	MaxRestarts int
	// StopGrace is the SIGTERM→SIGKILL window on Stop; defaults to 5s.
	StopGrace time.Duration
	// DetectPorts runs the lsof port-watch loop after each start.
	DetectPorts bool
	// OnStart, if set, is called before each (re)start (e.g. to write a log header).
	OnStart func()
	// OnStarted, if set, is called after each (re)start once the child is running
	// and is the current process — i.e. its Stdin()/StdoutReader() are available.
	// Used by JSON-RPC providers to (re)bind their client to the fresh child's
	// stdio and re-send the initialize handshake. It is invoked synchronously on
	// the supervise loop, so blocking work (e.g. awaiting a handshake response)
	// MUST be done in a goroutine, otherwise resource monitoring stalls.
	OnStarted func(*Process)
	// OnExit, if set, is called once when the supervise loop ends permanently
	// (the process exited and will not be restarted, or it was stopped).
	OnExit func()
}

// SupervisedProcess supervises a single process: it (re)runs a template command
// per its RestartPolicy, detects listening ports, samples CPU/memory/open-files
// of the whole process group, and enforces resource limits. It is created from a
// configured Process via (*Process).Supervise and driven with Start/Stop/Restart.
type SupervisedProcess struct {
	template *Process
	opts     SuperviseOptions

	mu          sync.RWMutex
	current     *Process // the running clone; nil between runs
	loopActive  bool
	desired     bool
	stopping    bool
	status      Status
	started     *time.Time
	restarts    int
	exitCode    *int
	ports       []int
	expectsPort bool // sticky: has bound a port before, so a (re)start gates on it
	gen         int
	done        chan struct{} // closed when the current supervise loop ends
	wake        chan struct{} // wakes the loop out of restart backoff

	// resource-monitor state (guarded by the same mu)
	latest  ResourceSnapshot
	peak    ResourceSnapshot
	tree    []ProcessSample
	highCPU int
	killed  bool
	// handles caches one gopsutil Process per pid so Percent(0) yields the CPU
	// delta since the previous sample rather than the lifetime average.
	handles map[int32]*gops.Process
}

// Supervise turns a configured Process into a SupervisedProcess using it as the
// template re-run on each (re)start. Call Start to begin supervising.
func (p *Process) Supervise(opts SuperviseOptions) *SupervisedProcess {
	if opts.Limits.Interval <= 0 {
		opts.Limits.Interval = defaultSampleInterval
	}
	if opts.Limits.CPUSampleCount <= 0 {
		opts.Limits.CPUSampleCount = defaultCPUSampleCount
	}
	if opts.StopGrace <= 0 {
		opts.StopGrace = defaultStopGrace
	}
	if opts.RestartPolicy == "" {
		opts.RestartPolicy = RestartNo
	}
	return &SupervisedProcess{
		template: p,
		opts:     opts,
		status:   StatusStopped,
		handles:  map[int32]*gops.Process{},
	}
}

// Supervise builds a supervised process from a command, mirroring NewExec.
func Supervise(opts SuperviseOptions, cmd string, args ...string) *SupervisedProcess {
	return NewExec(cmd, args...).Supervise(opts)
}

// --- state accessors -------------------------------------------------------

// Resources returns the most recent aggregate resource sample.
func (s *SupervisedProcess) Resources() ResourceSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latest
}

// Peak returns the high-water mark across all samples.
func (s *SupervisedProcess) Peak() ResourceSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peak
}

// Tree returns the most recent per-process breakdown of the supervised group.
func (s *SupervisedProcess) Tree() []ProcessSample {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]ProcessSample(nil), s.tree...)
}

// Killed reports whether the process was killed for breaching a resource limit.
func (s *SupervisedProcess) Killed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.killed
}

// Status returns the current lifecycle state.
func (s *SupervisedProcess) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// Ports returns the listening ports detected for the current run (empty if none
// or port detection is disabled).
func (s *SupervisedProcess) Ports() []int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]int(nil), s.ports...)
}

// Restarts returns the number of automatic restarts in the current run streak.
func (s *SupervisedProcess) Restarts() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.restarts
}

// ExitCode returns the last run's exit code, or nil if it hasn't exited yet.
func (s *SupervisedProcess) ExitCode() *int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ptrCopy(s.exitCode)
}

// Started returns when the current run started, or nil if not running.
func (s *SupervisedProcess) Started() *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ptrCopy(s.started)
}

// Pid returns the current run's pid, or 0 between runs.
func (s *SupervisedProcess) Pid() int {
	s.mu.RLock()
	c := s.current
	s.mu.RUnlock()
	if c == nil {
		return 0
	}
	return c.Pid()
}

// IsRunning reports whether a run is currently executing.
func (s *SupervisedProcess) IsRunning() bool {
	s.mu.RLock()
	c := s.current
	s.mu.RUnlock()
	return c != nil && c.IsRunning()
}

// KillTree force-kills the current run's process tree (no-op between runs).
func (s *SupervisedProcess) KillTree() error {
	s.mu.RLock()
	c := s.current
	s.mu.RUnlock()
	if c == nil {
		return nil
	}
	return c.KillTree()
}

// Name returns the template command's display name.
func (s *SupervisedProcess) Name() string {
	return s.template.Name()
}

func ptrCopy[T any](v *T) *T {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}

// --- resource monitor ------------------------------------------------------

// sample measures the process group once and records + enforces the result.
func (s *SupervisedProcess) sample() {
	pid := s.Pid()
	if pid <= 0 {
		return
	}
	pids := collectPids(int32(pid))
	fds := openFilesByPid(pids) // nil when the platform can't report fds

	var cpu float64
	var rss uint64
	openTotal := 0
	tree := make([]ProcessSample, 0, len(pids))
	for _, p := range pids {
		h := s.handle(p)
		if h == nil {
			continue
		}
		node := ProcessSample{PID: p, OpenFiles: -1}
		if ppid, err := h.Ppid(); err == nil {
			node.PPID = ppid
		}
		if name, err := h.Name(); err == nil {
			node.Command = name
		}
		if pct, err := h.Percent(0); err == nil {
			node.CPUPercent = pct
			cpu += pct
		}
		if mi, err := h.MemoryInfo(); err == nil && mi != nil {
			node.RSSBytes = mi.RSS
			rss += mi.RSS
		}
		if fds != nil {
			node.OpenFiles = fds[p]
			openTotal += fds[p]
		}
		tree = append(tree, node)
	}
	s.prune(pids)

	openAgg := -1 // sentinel: platform can't report open files
	if fds != nil {
		openAgg = openTotal
	}
	snap := ResourceSnapshot{
		CPUPercent: cpu,
		RSSBytes:   rss,
		OpenFiles:  openAgg,
		SampledAt:  time.Now(),
	}
	s.store(snap, tree)
	s.enforce(snap)
}

// collectPids returns the pids whose resource usage is attributed to this
// process. When the process leads its own group (started WithProcessGroup,
// pgid == root) every member of that process group is included — this captures
// children that reparent away from the process (double-fork) but stay in the
// group, which a parent→child walk would miss. Otherwise it falls back to a
// descendant walk, which never over-counts unrelated processes that merely
// share the caller's group.
func collectPids(root int32) []int32 {
	if pids, ok := groupPids(root); ok {
		return pids
	}
	return treePids(root)
}

// treePids returns root and every descendant pid via the parent→child tree. It
// is the fallback for collectPids when group enumeration is unavailable (the
// process is not a group leader, or off-unix). A failure to enumerate children
// (e.g. the process just exited) degrades to the pids found so far.
func treePids(root int32) []int32 {
	out := []int32{root}
	proc, err := gops.NewProcess(root)
	if err != nil {
		return out
	}
	var walk func(p *gops.Process)
	walk = func(p *gops.Process) {
		kids, err := p.Children()
		if err != nil {
			return
		}
		for _, k := range kids {
			out = append(out, k.Pid)
			walk(k)
		}
	}
	walk(proc)
	return out
}

func (s *SupervisedProcess) handle(pid int32) *gops.Process {
	s.mu.Lock()
	defer s.mu.Unlock()
	if h, ok := s.handles[pid]; ok {
		return h
	}
	h, err := gops.NewProcess(pid)
	if err != nil {
		return nil
	}
	s.handles[pid] = h
	return h
}

// prune drops cached handles for pids no longer in the tree so Percent deltas
// stay accurate and the map doesn't grow unbounded across many short children.
func (s *SupervisedProcess) prune(live []int32) {
	set := make(map[int32]struct{}, len(live))
	for _, p := range live {
		set[p] = struct{}{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for pid := range s.handles {
		if _, ok := set[pid]; !ok {
			delete(s.handles, pid)
		}
	}
}

func (s *SupervisedProcess) store(snap ResourceSnapshot, tree []ProcessSample) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latest = snap
	s.tree = tree
	if snap.CPUPercent > s.peak.CPUPercent {
		s.peak.CPUPercent = snap.CPUPercent
	}
	if snap.RSSBytes > s.peak.RSSBytes {
		s.peak.RSSBytes = snap.RSSBytes
	}
	if s.peak.SampledAt.IsZero() || snap.OpenFiles > s.peak.OpenFiles {
		s.peak.OpenFiles = snap.OpenFiles
	}
	s.peak.SampledAt = snap.SampledAt
}

// enforce kills the process tree when a configured limit is breached. RSS is
// enforced immediately; CPU only after CPUSampleCount consecutive over-limit
// samples so a brief startup spike doesn't trip it.
func (s *SupervisedProcess) enforce(snap ResourceSnapshot) {
	limits := s.opts.Limits
	if !limits.enabled() {
		return
	}
	if limits.MaxRSSBytes > 0 && snap.RSSBytes > limits.MaxRSSBytes {
		s.killForLimit("RSS %d bytes exceeds limit %d", snap.RSSBytes, limits.MaxRSSBytes)
		return
	}
	if limits.MaxCPUPercent <= 0 {
		return
	}
	s.mu.Lock()
	if snap.CPUPercent > limits.MaxCPUPercent {
		s.highCPU++
	} else {
		s.highCPU = 0
	}
	over := s.highCPU >= limits.CPUSampleCount
	s.mu.Unlock()
	if over {
		s.killForLimit("CPU %.1f%% exceeded limit %.1f%% for %d samples",
			snap.CPUPercent, limits.MaxCPUPercent, limits.CPUSampleCount)
	}
}

func (s *SupervisedProcess) killForLimit(format string, args ...any) {
	s.mu.Lock()
	if s.killed {
		s.mu.Unlock()
		return
	}
	s.killed = true
	s.mu.Unlock()
	log.Warnf("supervised process %s killed: "+format, append([]any{s.Name()}, args...)...)
	_ = s.KillTree()
}
