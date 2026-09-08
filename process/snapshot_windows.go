//go:build windows

package process

import (
	"context"
	"strings"
	"time"

	gops "github.com/shirou/gopsutil/v3/process"
)

func Discover(ctx context.Context, opts SnapshotOptions) (*Snapshot, error) {
	handles, err := gops.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	snapshot := &Snapshot{processes: make(map[int]Process), children: make(map[int][]int)}
	for _, handle := range handles {
		process := processFromHandle(ctx, handle)
		snapshot.processes[process.PID] = process
		snapshot.children[process.PPID] = append(snapshot.children[process.PPID], process.PID)
	}
	if opts.Environment != nil {
		populateEnvironments(ctx, snapshot, *opts.Environment)
	}
	return snapshot, nil
}

func processFromHandle(ctx context.Context, handle *gops.Process) Process {
	process := Process{PID: int(handle.Pid), Active: true}
	if ppid, err := handle.PpidWithContext(ctx); err == nil {
		process.PPID = int(ppid)
	}
	if status, err := handle.StatusWithContext(ctx); err == nil {
		process.Status = strings.Join(status, ",")
	}
	if cpu, err := handle.CPUPercentWithContext(ctx); err == nil {
		process.CPUPercent = cpu
	}
	if memory, err := handle.MemoryPercentWithContext(ctx); err == nil {
		process.MemoryPercent = float64(memory)
	}
	if memory, err := handle.MemoryInfoWithContext(ctx); err == nil && memory != nil {
		process.RSSBytes = memory.RSS
	}
	if started, err := handle.CreateTimeWithContext(ctx); err == nil {
		value := time.UnixMilli(started).UTC()
		process.StartedAt = &value
	}
	if command, err := handle.CmdlineWithContext(ctx); err == nil {
		process.Command = command
	}
	return process
}
