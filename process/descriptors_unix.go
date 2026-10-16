//go:build !windows

package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

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
