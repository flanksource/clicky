//go:build !unix

package exec

// groupPids is unsupported off unix (no POSIX process groups); callers fall
// back to a parent→child descendant walk.
func groupPids(root int32) ([]int32, bool) { return nil, false }
