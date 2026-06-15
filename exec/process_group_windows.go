//go:build windows

package exec

import (
	"os/exec"
	"syscall"
)

// CREATE_NEW_PROCESS_GROUP is defined in golang.org/x/sys/windows but we only
// need the constant here so we avoid pulling in that dep just for one symbol.
const createNewProcessGroup = 0x00000200

func applyProcessGroup(cmd *exec.Cmd, newPG bool) {
	if !newPG {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createNewProcessGroup
}

// killTree on Windows uses gopsutil's descendant walk for both the
// process-group and fallback paths. Job Object integration is a follow-up;
// today this delivers the same reliability guarantee as a two-pass walk and
// is cross-tested alongside the POSIX path.
func killTree(pid int, _ bool) error {
	if pid <= 0 {
		return nil
	}
	return killTreeByWalk(pid)
}

// terminateTree has no graceful equivalent on Windows (no POSIX signals); fall
// back to the same descendant kill so Stop still tears the tree down.
func terminateTree(pid int, _ bool) error {
	if pid <= 0 {
		return nil
	}
	return killTreeByWalk(pid)
}
