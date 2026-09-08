//go:build !windows

package process

import (
	"context"
	"os/exec"
)

const psFormat = "pid=,ppid=,pcpu=,pmem=,rss=,stat=,lstart=,command="

func Discover(ctx context.Context, opts SnapshotOptions) (*Snapshot, error) {
	output, err := exec.CommandContext(ctx, "ps", "-eo", psFormat).Output()
	if err != nil {
		return nil, err
	}
	snapshot := parseSnapshot(output, nil)
	if opts.Environment != nil {
		populateEnvironments(ctx, snapshot, *opts.Environment)
	}
	return snapshot, nil
}
