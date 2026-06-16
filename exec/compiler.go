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

func detectCompilers(root int32) (bool, error) {
	for _, pid := range collectPids(root) {
		proc, err := gops.NewProcess(pid)
		if err != nil {
			continue
		}
		if name, err := proc.Name(); err == nil && isCompilerExecutable(name) {
			return true, nil
		}
		if exe, err := proc.Exe(); err == nil && isCompilerExecutable(exe) {
			return true, nil
		}
		if args, err := proc.CmdlineSlice(); err == nil && isCompilerCommandLine(args) {
			return true, nil
		}
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
