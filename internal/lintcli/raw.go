package lintcli

import (
	"os"
	"strings"

	"github.com/flanksource/clicky/lint"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func SourceFlagPresent(args []string) bool {
	for _, arg := range args {
		if arg == "--source" || strings.HasPrefix(arg, "--source=") || arg == "--source-lines" || strings.HasPrefix(arg, "--source-lines=") {
			return true
		}
	}
	return false
}

func HelpRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

// RawArgs reports whether args select the raw go/analysis driver and, if so,
// returns the arguments to hand to it.
func RawArgs(args []string) ([]string, bool) {
	if len(args) > 0 && args[0] == "--" {
		return defaultRawPackages(args[1:]), true
	}
	for i, arg := range args {
		if arg == "--" {
			return defaultRawPackages(args[i+1:]), true
		}
		if arg == "--raw" {
			return withoutArg(args, i), true
		}
		if strings.HasPrefix(arg, "--raw=") {
			value := strings.TrimSpace(strings.TrimPrefix(arg, "--raw="))
			if value == "true" || value == "1" {
				return withoutArg(args, i), true
			}
		}
		if isAnalyzerDriverFlag(arg) {
			return defaultRawPackages(args), true
		}
	}
	return nil, false
}

func withoutArg(args []string, i int) []string {
	rawArgs := append([]string{}, args[:i]...)
	rawArgs = append(rawArgs, args[i+1:]...)
	if len(rawArgs) > 0 && rawArgs[0] == "--" {
		rawArgs = rawArgs[1:]
	}
	return defaultRawPackages(rawArgs)
}

func defaultRawPackages(args []string) []string {
	if len(args) == 0 {
		return []string{"./..."}
	}
	return args
}

func isAnalyzerDriverFlag(arg string) bool {
	if arg == "" || arg == "-h" || arg == "--help" {
		return false
	}
	if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
		return true
	}
	if !strings.HasPrefix(arg, "--") {
		return false
	}
	name := strings.TrimPrefix(arg, "--")
	if idx := strings.Index(name, "="); idx >= 0 {
		name = name[:idx]
	}
	switch name {
	case "c", "context", "cpuprofile", "debug", "diff", "fix", "flags", "json", "memprofile", "test", "trace", "V":
		return true
	default:
		return false
	}
}

// RunRaw hands args to the go/analysis singlechecker driver, which exits the process.
func RunRaw(args []string) error {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = append([]string{"clickylint"}, args...)
	singlechecker.Main(lint.Analyzer)
	return nil
}
