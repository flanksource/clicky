//go:build !darwin && !windows

package exec

import (
	"slices"

	"github.com/shirou/gopsutil/v3/process"
)

func pidIsZombie(pid int) (bool, error) {
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return false, err
	}
	statuses, err := proc.Status()
	return slices.Contains(statuses, process.Zombie), err
}
