//go:build unix

package exec

import (
	"syscall"

	gops "github.com/shirou/gopsutil/v3/process"
)

// groupPids lists every process in root's POSIX process group, but only when
// root is the group leader (pgid == root) — i.e. it was started
// WithProcessGroup. That guard prevents enumerating unrelated processes that
// merely share the caller's group. Returns (nil, false) to fall back to a
// descendant walk when root does not lead a group or the process table can't be
// read.
//
// This is O(processes on the host) per sample (one Getpgid syscall each), which
// is fine at the monitor's multi-second cadence for the handful of supervised
// processes a supervisor runs.
func groupPids(root int32) ([]int32, bool) {
	pgid, err := syscall.Getpgid(int(root))
	if err != nil || pgid != int(root) {
		return nil, false
	}
	all, err := gops.Pids()
	if err != nil {
		return nil, false
	}
	out := make([]int32, 0, 8)
	for _, pid := range all {
		if pg, err := syscall.Getpgid(int(pid)); err == nil && pg == pgid {
			out = append(out, pid)
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}
