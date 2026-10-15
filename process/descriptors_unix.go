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

// Descriptors lists the files and sockets pid holds open, read with lsof.
func Descriptors(ctx context.Context, pid int) ([]Descriptor, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "lsof", "-nP", "-a", "-p", strconv.Itoa(pid), "-F", "ftPnT")
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	descriptors := parseLsof(stdout.Bytes())
	// lsof exits 1 when it could not read some file of a process it still
	// listed; only an exit with nothing listed means the pid is unreadable.
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return nil, fmt.Errorf("lsof -p %d: %w: %s", pid, err, strings.TrimSpace(stderr.String()))
		}
		if len(descriptors) == 0 {
			return nil, fmt.Errorf("lsof: no open files for pid %d (exited, or not readable by this user): %s", pid, strings.TrimSpace(stderr.String()))
		}
	}
	return descriptors, nil
}
