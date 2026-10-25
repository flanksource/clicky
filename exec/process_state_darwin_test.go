//go:build darwin

package exec

import "golang.org/x/sys/unix"

const darwinZombieState = 5

func pidIsZombie(pid int) (bool, error) {
	state, err := darwinProcessState(pid)
	return state == darwinZombieState, err
}

func darwinProcessState(pid int) (int8, error) {
	process, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return 0, err
	}
	return process.Proc.P_stat, nil
}
