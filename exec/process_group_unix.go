//go:build !windows

package exec

import (
	"errors"
	"os/exec"
	"syscall"
)

func applyProcessGroup(cmd *exec.Cmd, newPG bool) {
	if !newPG {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// terminateTree asks the process tree to exit gracefully (SIGTERM). With a
// process group the signal reaches the whole group atomically (negative pid =
// pgid); otherwise it signals just the leader. Callers escalate to killTree
// (SIGKILL) if the tree doesn't exit within a grace window.
func terminateTree(pid int, hasProcessGroup bool) error {
	if pid <= 0 {
		return nil
	}
	target := pid
	if hasProcessGroup {
		target = -pid
	}
	if err := syscall.Kill(target, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil // already gone
		}
		return err
	}
	return nil
}

func killTree(pid int, hasProcessGroup bool) error {
	if pid <= 0 {
		return nil
	}
	if hasProcessGroup {
		// Atomic: SIGKILL every process in the group. Negative pid means pgid.
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				// Already gone — not an error.
				return nil
			}
			return err
		}
		return nil
	}
	// Fallback: racy descendant walk via gopsutil. Kept minimal — callers
	// should prefer WithProcessGroup() for reliable tree kills.
	return killTreeByWalk(pid)
}
