//go:build darwin

package exec

import "golang.org/x/sys/unix"

const darwinZombieState = 5

func pidIsZombie(pid int) (bool, error) {
	process, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return false, err
	}
	return process.Proc.P_stat == darwinZombieState, nil
}
