//go:build darwin

package exec

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

// openFilesByPid returns the open-file count per pid via a single lsof call,
// since gopsutil does not implement NumFDs/OpenFiles on darwin. With -F pf lsof
// prints a `p<pid>` record starting each process block followed by one `f<…>`
// record per open file (numeric fds plus cwd/txt/mem), so counting f-records
// against the current p-record yields per-pid totals. Returns nil when lsof is
// unavailable so callers can render "unsupported" rather than a misleading 0.
func openFilesByPid(pids []int32) map[int32]int {
	if len(pids) == 0 {
		return map[int32]int{}
	}
	ids := make([]string, len(pids))
	for i, pid := range pids {
		ids[i] = strconv.Itoa(int(pid))
	}
	bin, err := exec.LookPath("lsof")
	if err != nil {
		return nil
	}
	// lsof exits non-zero when some pids are already gone but still prints the
	// rest, so the output is parsed regardless of the exit status.
	out, err := exec.Command(bin, "-p", strings.Join(ids, ","), "-F", "pf").Output()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return nil
	}
	counts := make(map[int32]int, len(pids))
	if len(out) == 0 {
		return counts
	}
	cur := int32(-1)
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			if v, err := strconv.Atoi(line[1:]); err == nil {
				cur = int32(v)
				counts[cur] = 0
			}
		case 'f':
			if cur >= 0 {
				counts[cur]++
			}
		}
	}
	if len(counts) == 0 {
		return nil
	}
	return counts
}
