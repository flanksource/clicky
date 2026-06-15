//go:build linux

package exec

import (
	"os"
	"strconv"
)

// openFilesByPid returns the open-file-descriptor count per pid by reading
// /proc/<pid>/fd. Pids that have already exited are omitted. The map is always
// non-nil on linux (the platform can always report fds), so callers treat a
// missing pid as zero rather than "unsupported".
func openFilesByPid(pids []int32) map[int32]int {
	out := make(map[int32]int, len(pids))
	for _, pid := range pids {
		entries, err := os.ReadDir("/proc/" + strconv.Itoa(int(pid)) + "/fd")
		if err != nil {
			continue
		}
		out[pid] = len(entries)
	}
	return out
}
