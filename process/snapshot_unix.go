//go:build !windows

package process

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const psFormat = "pid=,ppid=,pcpu=,pmem=,rss=,stat=,time=,lstart=,command="

// Discover lists the host's processes, leaving out the ps it runs to list
// them: a caller sampling its own process tree would otherwise see a new
// child of its own at every sample. ps runs under LC_ALL=C so lstart and
// decimal columns parse regardless of the caller's locale.
func Discover(ctx context.Context, opts SnapshotOptions) (*Snapshot, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ps", "-eo", psFormat)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ps: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	snapshot, err := parseSnapshot(stdout.Bytes(), nil)
	if err != nil {
		return nil, fmt.Errorf("ps: %w", err)
	}
	snapshot.remove(cmd.Process.Pid)
	if opts.Environment != nil {
		populateEnvironments(ctx, snapshot, *opts.Environment)
	}
	return snapshot, nil
}
