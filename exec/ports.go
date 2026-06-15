package exec

import (
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// ListeningPorts returns the distinct TCP ports the process group led by pgid is
// listening on, in ascending order. It shells out to lsof, which is present on
// macOS and most Linux dev/CI images. WithProcessGroup() puts each supervised
// process in its own group whose id equals the leader pid, so passing the leader
// pid as pgid captures listeners opened by the process and any child it forked
// (e.g. `npm run dev` → node).
//
// Detection is advisory: when lsof is not on PATH the result is (nil, nil) so
// callers degrade to "no ports detected" (and Windows, which has no lsof, is a
// no-op). lsof exits non-zero when nothing matches ("not listening yet") — that
// is the normal empty case, so whatever it printed is parsed instead of erroring.
// Only a failure to execute lsof at all is returned as an error.
func ListeningPorts(pgid int) ([]int, error) {
	if pgid <= 0 {
		return nil, nil
	}
	return runLsofListen("-g", strconv.Itoa(pgid))
}

func listeningPortsForPids(pids []int32) ([]int, error) {
	if len(pids) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(pids))
	for _, pid := range pids {
		if pid > 0 {
			ids = append(ids, strconv.Itoa(int(pid)))
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return runLsofListen("-p", strings.Join(ids, ","))
}

func runLsofListen(selector ...string) ([]int, error) {
	bin, err := exec.LookPath("lsof")
	if err != nil {
		return nil, nil
	}
	// -n/-P skip DNS and port-name lookups (numeric output); -a ANDs the
	// selector with the TCP-listen filter; -F n emits one machine-readable
	// `n<addr>` line per matching socket.
	args := []string{"-nP", "-a"}
	args = append(args, selector...)
	args = append(args, "-iTCP", "-sTCP:LISTEN", "-F", "n")
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return parseListenPorts(string(out)), nil
		}
		return nil, fmt.Errorf("run lsof %s: %w", strings.Join(selector, " "), err)
	}
	return parseListenPorts(string(out)), nil
}

// parseListenPorts extracts the listening ports from lsof `-F n` output. Each
// address field is on its own `n`-prefixed line, e.g. `n*:3000`,
// `n127.0.0.1:8080`, or `n[::1]:5000`; the port is the digits after the final
// colon. Established connections (which carry a `->` arrow) are ignored.
func parseListenPorts(out string) []int {
	seen := map[int]struct{}{}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 2 || line[0] != 'n' {
			continue
		}
		addr := strings.TrimSpace(line[1:])
		if strings.Contains(addr, "->") {
			continue
		}
		i := strings.LastIndex(addr, ":")
		if i < 0 {
			continue
		}
		port, err := strconv.Atoi(addr[i+1:])
		if err != nil || port <= 0 || port > 65535 {
			continue
		}
		seen[port] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports
}
