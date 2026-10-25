package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	gops "github.com/shirou/gopsutil/v4/process"
)

type EnvironmentOptions struct {
	Keys     []string
	Prefixes []string
}

type SnapshotOptions struct {
	Environment *EnvironmentOptions
}

type Process struct {
	PID    int    `json:"pid"`
	PPID   int    `json:"ppid,omitempty"`
	Status string `json:"status,omitempty"`
	Active bool   `json:"active"`
	// CPUPercent is the OS-reported figure, not current load: a lifetime
	// average (cpu time ÷ elapsed) on linux and windows, a decaying ~1 minute
	// average on darwin. A hung busy loop can read as idle; use CPUTime with
	// IntervalCPUPercent for the CPU% over a sampling interval.
	CPUPercent float64 `json:"cpuPercent,omitempty"`
	// CPUTime is the cumulative user+system CPU time the process has used
	// (JSON: nanoseconds). Resolution is 1s on linux ps, 10ms on darwin ps.
	CPUTime       time.Duration `json:"cpuTime,omitempty"`
	MemoryPercent float64       `json:"memoryPercent,omitempty"`
	RSSBytes      uint64        `json:"rssBytes,omitempty"`
	IO            *ProcessIO    `json:"io,omitempty"`
	IOError       string        `json:"ioError,omitempty"`
	StartedAt     *time.Time    `json:"startedAt,omitempty"`
	// UID is the process's owner and User its name, which linux ps cuts to
	// 8 characters.
	UID              int               `json:"uid"`
	User             string            `json:"user,omitempty"`
	Command          string            `json:"command,omitempty"`
	CWD              string            `json:"cwd,omitempty"`
	CWDError         string            `json:"cwdError,omitempty"`
	Environment      map[string]string `json:"environment,omitempty"`
	EnvironmentError string            `json:"environmentError,omitempty"`
}

type ProcessIO struct {
	DiskReadBytes  uint64 `json:"diskReadBytes"`
	DiskWriteBytes uint64 `json:"diskWriteBytes"`
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

// remove drops pid, and its place among its parent's children, from s.
func (s *Snapshot) remove(pid int) {
	process, found := s.processes[pid]
	if !found {
		return
	}
	delete(s.processes, pid)
	siblings := s.children[process.PPID]
	for i, sibling := range siblings {
		if sibling == pid {
			s.children[process.PPID] = append(siblings[:i:i], siblings[i+1:]...)
			break
		}
	}
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

func (s *Snapshot) PopulateIO(ctx context.Context, pids []int) error {
	return s.populateIO(ctx, pids, readProcessIO)
}

func readProcessIO(ctx context.Context, pid int) (*gops.IOCountersStat, error) {
	handle, err := gops.NewProcessWithContext(ctx, int32(pid))
	if err != nil {
		return nil, err
	}
	return handle.IOCountersWithContext(ctx)
}

func (s *Snapshot) populateIO(ctx context.Context, pids []int, readIO func(context.Context, int) (*gops.IOCountersStat, error)) error {
	for _, pid := range pids {
		item, found := s.processes[pid]
		// A zombie reads as all-zero counters and an exiting process as EPERM:
		// neither has disk counters left to report.
		if !found || item.Status == "zombie" || item.Status == "exiting" {
			continue
		}
		counters, err := readIO(ctx, pid)
		if err == nil {
			item.IO = &ProcessIO{DiskReadBytes: counters.DiskReadBytes, DiskWriteBytes: counters.DiskWriteBytes}
			item.IOError = ""
			s.processes[pid] = item
			continue
		}
		if errors.Is(err, gops.ErrorProcessNotRunning) || errors.Is(err, os.ErrNotExist) {
			continue
		}
		// Another user's or a setuid process (ps, sudo) cannot be read by the
		// caller, and any other errno the kernel gives is about that one
		// process too: that is the process's reading, not the snapshot's
		// failure.
		var errno syscall.Errno
		if errors.Is(err, gops.ErrorNotPermitted) || errors.Is(err, os.ErrPermission) || errors.As(err, &errno) {
			item.IO = nil
			item.IOError = fmt.Sprintf("read disk I/O: %v", err)
			s.processes[pid] = item
			continue
		}
		return fmt.Errorf("read disk I/O for process %d: %w", pid, err)
	}
	return nil
}

func parseSnapshot(output []byte, location *time.Location) (*Snapshot, error) {
	if location == nil {
		location = time.Local
	}
	var processes []Process
	for _, raw := range bytes.Split(output, []byte{'\n'}) {
		line := strings.TrimSpace(string(raw))
		if line == "" {
			continue
		}
		process, err := parseProcessLine(line, location)
		if err != nil {
			return nil, err
		}
		processes = append(processes, process)
	}
	return NewSnapshot(processes), nil
}

// parseProcessLine parses one row of psFormat; lstart spans five fields,
// and user one, as owner names hold no spaces.
func parseProcessLine(line string, location *time.Location) (Process, error) {
	const uidField, userField, commandField = 12, 13, 14
	fields := strings.Fields(line)
	if len(fields) <= commandField {
		return Process{}, fmt.Errorf("ps row has %d fields, want more than %d: %q", len(fields), commandField, line)
	}
	pid, pidErr := strconv.Atoi(fields[0])
	ppid, ppidErr := strconv.Atoi(fields[1])
	cpu, cpuErr := strconv.ParseFloat(fields[2], 64)
	memory, memoryErr := strconv.ParseFloat(fields[3], 64)
	rssKB, rssErr := strconv.ParseUint(fields[4], 10, 64)
	cpuTime, cpuTimeErr := parseCPUTime(fields[6])
	started, startedErr := time.ParseInLocation("Mon Jan 2 15:04:05 2006", strings.Join(fields[7:uidField], " "), location)
	uid, uidErr := strconv.Atoi(fields[uidField])
	if err := errors.Join(columnError("pid", pidErr), columnError("ppid", ppidErr), columnError("pcpu", cpuErr),
		columnError("pmem", memoryErr), columnError("rss", rssErr), columnError("time", cpuTimeErr), columnError("lstart", startedErr),
		columnError("uid", uidErr)); err != nil {
		return Process{}, fmt.Errorf("ps row %q: %w", line, err)
	}
	status, active := processStatus(fields[5])
	started = started.UTC()
	return Process{
		PID: pid, PPID: ppid, Status: status, Active: active,
		CPUPercent: cpu, CPUTime: cpuTime, MemoryPercent: memory, RSSBytes: rssKB * 1024,
		StartedAt: &started, UID: uid, User: fields[userField], Command: strings.Join(fields[commandField:], " "),
	}, nil
}

func columnError(column string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", column, err)
}

func processStatus(status string) (string, bool) {
	switch {
	case strings.Contains(status, "Z"):
		return "zombie", false
	// BSD "E" (trying to exit) and linux "X" (dead): ps may already report the
	// process's cpu time as 0, so it must not be rated like a live process.
	case strings.Contains(status, "E"), strings.Contains(status, "X"):
		return "exiting", false
	case strings.Contains(status, "T"):
		return "stopped", false
	case strings.Contains(status, "S"):
		return "sleeping", true
	default:
		return "active", true
	}
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
