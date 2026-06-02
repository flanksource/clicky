package lint

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"
)

// RunOptions controls the standalone clickylint runner used by the clicky CLI.
type RunOptions struct {
	Packages     []string `json:"packages,omitempty"`
	WorkDir      string   `json:"work_dir,omitempty"`
	IncludeTests bool     `json:"include_tests"`
}

// Result is the normalized lint result rendered by the CLI and JSON output.
type Result struct {
	Linter       string        `json:"linter"`
	WorkDir      string        `json:"work_dir,omitempty"`
	PackageCount int           `json:"package_count"`
	Success      bool          `json:"success"`
	Duration     time.Duration `json:"duration"`
	Violations   []Violation   `json:"violations,omitempty"`
	Errors       []string      `json:"errors,omitempty"`
}

// Violation is a display-oriented analysis diagnostic.
type Violation struct {
	Package string `json:"package,omitempty"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	Rule    string `json:"rule,omitempty"`
	Message string `json:"message,omitempty"`
}

// HasIssues reports whether the lint run found diagnostics or execution errors.
func (r *Result) HasIssues() bool {
	if r == nil {
		return false
	}
	return len(r.Violations) > 0 || len(r.Errors) > 0
}

// Run loads the requested Go packages, runs clickylint, and returns normalized
// diagnostics without printing through the go/analysis default text driver.
func Run(opts RunOptions) (*Result, error) {
	if len(opts.Packages) == 0 {
		opts.Packages = []string{"./..."}
	}
	if opts.WorkDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		opts.WorkDir = wd
	}

	start := time.Now()
	result := &Result{
		Linter:  Analyzer.Name,
		WorkDir: opts.WorkDir,
	}

	cfg := &packages.Config{
		Mode:  packages.LoadSyntax | packages.NeedModule,
		Dir:   opts.WorkDir,
		Tests: opts.IncludeTests,
	}
	pkgs, err := packages.Load(cfg, opts.Packages...)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		result.Duration = time.Since(start)
		return result, nil
	}
	if len(pkgs) == 0 {
		result.Errors = append(result.Errors, fmt.Sprintf("%s matched no packages", strings.Join(opts.Packages, " ")))
		result.Duration = time.Since(start)
		return result, nil
	}

	result.PackageCount = len(pkgs)
	result.Errors = append(result.Errors, packageErrors(pkgs)...)

	graph, err := checker.Analyze([]*analysis.Analyzer{Analyzer}, pkgs, nil)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		result.Duration = time.Since(start)
		return result, nil
	}

	for _, act := range graph.Roots {
		if act == nil || act.Analyzer != Analyzer {
			continue
		}
		if act.Err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", act.Package.PkgPath, act.Err))
			continue
		}
		for _, diag := range act.Diagnostics {
			pos := act.Package.Fset.PositionFor(diag.Pos, false)
			result.Violations = append(result.Violations, Violation{
				Package: act.Package.PkgPath,
				File:    pos.Filename,
				Line:    pos.Line,
				Column:  pos.Column,
				Rule:    RuleForMessage(diag.Message),
				Message: diag.Message,
			})
		}
	}

	sortViolations(result.Violations)
	result.Errors = uniqueStrings(result.Errors)
	result.Success = !result.HasIssues()
	result.Duration = time.Since(start)
	return result, nil
}

// RuleForMessage derives a stable rule bucket from a go/analysis diagnostic.
func RuleForMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "(no rule)"
	}
	if idx := strings.Index(message, ";"); idx >= 0 {
		return strings.TrimSpace(message[:idx])
	}
	return message
}

func packageErrors(pkgs []*packages.Package) []string {
	var out []string
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		for _, err := range pkg.Errors {
			out = append(out, err.Error())
		}
	})
	return uniqueStrings(out)
}

func sortViolations(violations []Violation) {
	sort.SliceStable(violations, func(i, j int) bool {
		a, b := violations[i], violations[j]
		switch {
		case a.File != b.File:
			return a.File < b.File
		case a.Line != b.Line:
			return a.Line < b.Line
		case a.Column != b.Column:
			return a.Column < b.Column
		case a.Rule != b.Rule:
			return a.Rule < b.Rule
		default:
			return a.Message < b.Message
		}
	})
}

func uniqueStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}
