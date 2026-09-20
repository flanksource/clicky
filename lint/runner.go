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
	// Severity re-levels individual rules by ID, so a codebase can adopt a rule
	// as advice before it adopts it as a gate. Unknown IDs are reported as run
	// errors rather than ignored: a typo that silently kept a rule failing the
	// build would be worse than a loud one.
	Severity map[string]Severity `json:"severity,omitempty"`
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
	Package  string   `json:"package,omitempty"`
	File     string   `json:"file,omitempty"`
	Line     int      `json:"line,omitempty"`
	Column   int      `json:"column,omitempty"`
	Severity Severity `json:"severity,omitempty"`
	Rule     string   `json:"rule,omitempty"`
	Message  string   `json:"message,omitempty"`
}

// HasIssues reports whether the lint run found any diagnostics (error or
// warning) or execution errors. Used to decide whether output is worth showing.
func (r *Result) HasIssues() bool {
	if r == nil {
		return false
	}
	return len(r.Violations) > 0 || len(r.Errors) > 0
}

// HasErrors reports whether the run found error-severity violations or
// execution errors. This — not HasIssues — drives the non-zero exit status;
// warnings are advisory.
func (r *Result) HasErrors() bool {
	if r == nil {
		return false
	}
	return r.ErrorCount() > 0 || len(r.Errors) > 0
}

// ErrorCount counts error-severity violations.
func (r *Result) ErrorCount() int {
	return r.countSeverity(SeverityError)
}

// WarningCount counts warning-severity violations.
func (r *Result) WarningCount() int {
	return r.countSeverity(SeverityWarning)
}

func (r *Result) countSeverity(sev Severity) int {
	if r == nil {
		return 0
	}
	n := 0
	for _, v := range r.Violations {
		if v.Severity == sev {
			n++
		}
	}
	return n
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
	result.Errors = append(result.Errors, unknownSeverityRules(opts.Severity)...)

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
			severity, ruleID := parseCategory(diag.Category)
			if override, ok := opts.Severity[ruleID]; ok && ruleID != "" {
				severity = override
			}
			result.Violations = append(result.Violations, Violation{
				Package:  act.Package.PkgPath,
				File:     pos.Filename,
				Line:     pos.Line,
				Column:   pos.Column,
				Severity: severity,
				Rule:     ruleID,
				Message:  diag.Message,
			})
		}
	}

	sortViolations(result.Violations)
	result.Violations = dedupeViolations(result.Violations)
	result.Errors = uniqueStrings(result.Errors)
	result.Success = !result.HasErrors()
	result.Duration = time.Since(start)
	return result, nil
}

// unknownSeverityRules names overrides that address no rule, so a misspelled ID
// surfaces instead of quietly leaving the rule at its default.
func unknownSeverityRules(overrides map[string]Severity) []string {
	if len(overrides) == 0 {
		return nil
	}
	known := map[string]bool{}
	for _, id := range RuleIDs() {
		known[id] = true
	}
	var unknown []string
	for id := range overrides {
		if !known[id] {
			unknown = append(unknown, fmt.Sprintf("unknown rule %q in severity override; known rules: %s",
				id, strings.Join(RuleIDs(), ", ")))
		}
	}
	sort.Strings(unknown)
	return unknown
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

// dedupeViolations collapses the same finding reported more than once. Loading
// with tests returns a package and its test variant as separate packages that
// share a path and both contain the production files, so every finding in a
// package that has tests is analyzed — and reported — twice. Callers count
// violations to decide whether a codebase is improving; double counts make that
// number meaningless. Expects violations already sorted, so duplicates adjoin.
func dedupeViolations(violations []Violation) []Violation {
	if len(violations) < 2 {
		return violations
	}
	unique := violations[:1]
	for _, violation := range violations[1:] {
		if violation != unique[len(unique)-1] {
			unique = append(unique, violation)
		}
	}
	return unique
}

func sortViolations(violations []Violation) {
	sort.SliceStable(violations, func(i, j int) bool {
		a, b := violations[i], violations[j]
		switch {
		case a.Package != b.Package:
			return a.Package < b.Package
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
