//go:build !windows

package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const psFormat = "pid=,ppid=,pcpu=,pmem=,rss=,stat=,time=,lstart=,uid=,user=,command="

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

// Descriptors lists the files and sockets each of pids holds open, by pid,
// read with one lsof call. A pid lsof lists nothing for, one that is not
// running or not readable by this user, is left out.
func Descriptors(ctx context.Context, pids ...int) (map[int][]Descriptor, error) {
	if len(pids) == 0 {
		return map[int][]Descriptor{}, nil
	}
	list := make([]string, 0, len(pids))
	for _, pid := range pids {
		list = append(list, strconv.Itoa(pid))
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "lsof", "-nP", "-a", "-p", strings.Join(list, ","), "-F", "pftPnT")
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	// lsof exits 1 when some pid is not running, or some file of a process it
	// still listed could not be read: only a failure to run it is an error.
	var exitErr *exec.ExitError
	if err != nil && (!errors.As(err, &exitErr) || ctx.Err() != nil) {
		return nil, fmt.Errorf("lsof -p %s: %w: %s", strings.Join(list, ","), err, strings.TrimSpace(stderr.String()))
	}
	return parseLsof(stdout.Bytes()), nil
}
