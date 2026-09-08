package process

import (
	"bytes"
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	gops "github.com/shirou/gopsutil/v3/process"
)

type EnvironmentOptions struct {
	Keys     []string
	Prefixes []string
}

type SnapshotOptions struct {
	Environment *EnvironmentOptions
}

type Process struct {
	PID              int               `json:"pid"`
	PPID             int               `json:"ppid,omitempty"`
	Status           string            `json:"status,omitempty"`
	Active           bool              `json:"active"`
	CPUPercent       float64           `json:"cpuPercent,omitempty"`
	MemoryPercent    float64           `json:"memoryPercent,omitempty"`
	RSSBytes         uint64            `json:"rssBytes,omitempty"`
	StartedAt        *time.Time        `json:"startedAt,omitempty"`
	Command          string            `json:"command,omitempty"`
	CWD              string            `json:"cwd,omitempty"`
	CWDError         string            `json:"cwdError,omitempty"`
	Environment      map[string]string `json:"environment,omitempty"`
	EnvironmentError string            `json:"environmentError,omitempty"`
}

type ResourceUsage struct {
	CPUPercent    float64   `json:"cpuPercent"`
	MemoryPercent float64   `json:"memoryPercent"`
	RSSBytes      uint64    `json:"rssBytes"`
	Processes     []Process `json:"processes,omitempty"`
}

type Snapshot struct {
	processes map[int]Process
	children  map[int][]int
}

func NewSnapshot(processes []Process) *Snapshot {
	snapshot := &Snapshot{processes: make(map[int]Process), children: make(map[int][]int)}
	for _, process := range processes {
		if process.PID <= 0 {
			continue
		}
		snapshot.processes[process.PID] = process
		snapshot.children[process.PPID] = append(snapshot.children[process.PPID], process.PID)
	}
	for parent := range snapshot.children {
		sort.Ints(snapshot.children[parent])
	}
	return snapshot
}

func (s *Snapshot) All() []Process {
	pids := make([]int, 0, len(s.processes))
	for pid := range s.processes {
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	processes := make([]Process, 0, len(pids))
	for _, pid := range pids {
		processes = append(processes, s.processes[pid])
	}
	return processes
}

func (s *Snapshot) Get(pid int) (Process, bool) {
	process, found := s.processes[pid]
	return process, found
}

func (s *Snapshot) Subtree(pid int) []Process {
	root, found := s.processes[pid]
	if !found {
		return nil
	}
	visited := map[int]bool{pid: true}
	processes := []Process{root}
	for queue := []int{pid}; len(queue) > 0; {
		current := queue[0]
		queue = queue[1:]
		for _, childPID := range s.children[current] {
			if visited[childPID] {
				continue
			}
			child, found := s.processes[childPID]
			if !found {
				continue
			}
			visited[childPID] = true
			processes = append(processes, child)
			queue = append(queue, childPID)
		}
	}
	return processes
}

func (s *Snapshot) Ancestors(pid int) []Process {
	process, found := s.processes[pid]
	if !found {
		return nil
	}
	visited := map[int]bool{pid: true}
	var ancestors []Process
	for current := process.PPID; !visited[current]; {
		parent, found := s.processes[current]
		if !found {
			break
		}
		visited[current] = true
		ancestors = append(ancestors, parent)
		current = parent.PPID
	}
	return ancestors
}

func (s *Snapshot) AggregateSubtree(pid int) ResourceUsage {
	var usage ResourceUsage
	for _, process := range s.Subtree(pid) {
		usage.CPUPercent += process.CPUPercent
		usage.MemoryPercent += process.MemoryPercent
		usage.RSSBytes += process.RSSBytes
		usage.Processes = append(usage.Processes, process)
	}
	return usage
}

func (s *Snapshot) PopulateWorkingDirectories(ctx context.Context, pids []int) {
	for _, pid := range pids {
		process, found := s.processes[pid]
		if !found {
			continue
		}
		handle, err := gops.NewProcessWithContext(ctx, int32(pid))
		if err != nil {
			process.CWDError = err.Error()
			s.processes[pid] = process
			continue
		}
		cwd, err := handle.CwdWithContext(ctx)
		if err != nil {
			process.CWDError = err.Error()
			s.processes[pid] = process
			continue
		}
		process.CWD = cwd
		s.processes[pid] = process
	}
}

func parseSnapshot(output []byte, location *time.Location) *Snapshot {
	if location == nil {
		location = time.Local
	}
	var processes []Process
	for _, raw := range bytes.Split(output, []byte{'\n'}) {
		process, found := parseProcessLine(strings.TrimSpace(string(raw)), location)
		if !found {
			continue
		}
		processes = append(processes, process)
	}
	return NewSnapshot(processes)
}

func parseProcessLine(line string, location *time.Location) (Process, bool) {
	const processFields = 11
	fields := strings.Fields(line)
	if len(fields) <= processFields {
		return Process{}, false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil || pid <= 0 {
		return Process{}, false
	}
	ppid, _ := strconv.Atoi(fields[1])
	cpu, _ := strconv.ParseFloat(fields[2], 64)
	memory, _ := strconv.ParseFloat(fields[3], 64)
	rssKB, _ := strconv.ParseUint(fields[4], 10, 64)
	status, active := processStatus(fields[5])
	return Process{
		PID: pid, PPID: ppid, Status: status, Active: active,
		CPUPercent: cpu, MemoryPercent: memory, RSSBytes: rssKB * 1024,
		StartedAt: parseProcessStart(strings.Join(fields[6:processFields], " "), location),
		Command:   strings.Join(fields[processFields:], " "),
	}, true
}

func processStatus(status string) (string, bool) {
	switch {
	case strings.Contains(status, "Z"):
		return "zombie", false
	case strings.Contains(status, "T"):
		return "stopped", false
	case strings.Contains(status, "S"):
		return "sleeping", true
	default:
		return "active", true
	}
}

func parseProcessStart(value string, location *time.Location) *time.Time {
	parsed, err := time.ParseInLocation("Mon Jan 2 15:04:05 2006", value, location)
	if err != nil {
		return nil
	}
	utc := parsed.UTC()
	return &utc
}

func populateEnvironments(ctx context.Context, snapshot *Snapshot, opts EnvironmentOptions) {
	for pid, process := range snapshot.processes {
		environment, err := readEnvironment(ctx, pid)
		if err != nil {
			process.EnvironmentError = err.Error()
		} else {
			process.Environment = filterEnvironment(environment, opts)
		}
		snapshot.processes[pid] = process
	}
}

func filterEnvironment(entries []string, opts EnvironmentOptions) map[string]string {
	environment := make(map[string]string)
	for _, entry := range entries {
		key, value, found := strings.Cut(entry, "=")
		if !found || key == "" || !environmentSelected(key, opts) {
			continue
		}
		environment[key] = value
	}
	return environment
}

func environmentSelected(key string, opts EnvironmentOptions) bool {
	if len(opts.Keys) == 0 && len(opts.Prefixes) == 0 {
		return true
	}
	for _, candidate := range opts.Keys {
		if key == candidate {
			return true
		}
	}
	for _, prefix := range opts.Prefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}
