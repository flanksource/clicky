//go:build !windows

package process

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const psFormat = "pid=,ppid=,pcpu=,pmem=,rss=,stat=,lstart=,command="

// Discover lists the host's processes, leaving out the ps it runs to list
// them: a caller sampling its own process tree would otherwise see a new
// child of its own at every sample.
func Discover(ctx context.Context, opts SnapshotOptions) (*Snapshot, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ps", "-eo", psFormat)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ps: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	snapshot := parseSnapshot(stdout.Bytes(), nil)
	snapshot.remove(cmd.Process.Pid)
	if opts.Environment != nil {
		populateEnvironments(ctx, snapshot, *opts.Environment)
	}
	return snapshot, nil
}
