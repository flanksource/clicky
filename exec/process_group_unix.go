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
