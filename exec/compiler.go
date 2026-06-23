package exec

import (
	"path/filepath"
	"strings"

	gops "github.com/shirou/gopsutil/v3/process"
)

var compilerExecutableBasenames = map[string]struct{}{
	"asm":     {},
	"babel":   {},
	"cgo":     {},
	"compile": {},
	"esbuild": {},
	"link":    {},
	"parcel":  {},
	"rollup":  {},
	"swc":     {},
	"tsc":     {},
	"webpack": {},
}

var compilerArgumentBasenames = map[string]struct{}{
	"babel":   {},
	"esbuild": {},
	"parcel":  {},
	"rollup":  {},
	"swc":     {},
	"tsc":     {},
	"webpack": {},
}

// detectCompilers reports whether any process in the tree rooted at root is a
// compiler/linker. A nil error means the tree was inspected successfully (with
// or without a match). When every probe fails — no process in the tree could be
// inspected at all — it returns the first probe error so the caller can tell
// "no compiler running" apart from "couldn't see the process tree" and avoid
// advancing startup state on blind detection.
func detectCompilers(root int32) (bool, error) {
	var firstErr error
	visible := false
	noteErr := func(err error) {
		if firstErr == nil {
			firstErr = err
		}
	}
	for _, pid := range collectPids(root) {
		proc, err := gops.NewProcess(pid)
		if err != nil {
			noteErr(err)
			continue
		}
		if name, err := proc.Name(); err == nil {
			visible = true
			if isCompilerExecutable(name) {
				return true, nil
			}
		} else {
			noteErr(err)
		}
		if exe, err := proc.Exe(); err == nil {
			visible = true
			if isCompilerExecutable(exe) {
				return true, nil
			}
		} else {
			noteErr(err)
		}
		if args, err := proc.CmdlineSlice(); err == nil {
			visible = true
			if isCompilerCommandLine(args) {
				return true, nil
			}
		} else {
			noteErr(err)
		}
	}
	if !visible && firstErr != nil {
		return false, firstErr
	}
	return false, nil
}

func isCompilerCommandLine(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if isCompilerExecutable(args[0]) {
		return true
	}
	for _, arg := range args[1:] {
		if isCompilerArgument(arg) {
			return true
		}
	}
	return false
}

func isCompilerExecutable(value string) bool {
	return hasCompilerBasename(value, compilerExecutableBasenames)
}

func isCompilerArgument(value string) bool {
	return hasCompilerBasename(value, compilerArgumentBasenames)
}

func hasCompilerBasename(value string, matchers map[string]struct{}) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(value)))
	base = strings.TrimSuffix(base, ".exe")
	_, ok := matchers[base]
	return ok
}
