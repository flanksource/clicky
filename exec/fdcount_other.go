//go:build !linux && !darwin

package exec

// openFilesByPid is unsupported on this platform; nil signals "unknown" so the
// aggregate renders distinctly from a real zero.
func openFilesByPid(pids []int32) map[int32]int { return nil }
