package lint

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/flanksource/clicky/api"
)

// SummaryView renders clickylint results as a compact tree, matching the
// Gavel lint display shape: root -> linter -> rule -> affected files.
type SummaryView struct {
	Result *Result `json:"result"`
	Limit  int     `json:"-"`
}

// NewSummaryView returns a tree view for a lint result.
func NewSummaryView(result *Result, limit int) *SummaryView {
	if limit < 1 {
		limit = 5
	}
	return &SummaryView{Result: result, Limit: limit}
}

func (s *SummaryView) Pretty() api.Text {
	errs, warns, loadErrs := 0, 0, 0
	if s.Result != nil {
		errs = s.Result.ErrorCount()
		warns = s.Result.WarningCount()
		loadErrs = len(s.Result.Errors)
	}
	style := "text-blue-500"
	switch {
	case errs == 0 && warns == 0 && loadErrs == 0:
		style = "text-green-600"
	case errs > 0 || loadErrs > 0:
		style = "text-red-500"
	default:
		style = "text-yellow-600"
	}
	text := fmt.Sprintf("Lint summary: %d errors, %d warnings", errs, warns)
	if loadErrs > 0 {
		text += fmt.Sprintf(" (%d load errors)", loadErrs)
	}
	return api.Text{Content: text, Style: style}
}

func (s *SummaryView) GetChildren() []api.TreeNode {
	if s.Result == nil {
		return nil
	}
	return []api.TreeNode{&linterSummaryNode{
		linter:     s.Result.Linter,
		workDir:    s.Result.WorkDir,
		violations: s.Result.Violations,
		errors:     s.Result.Errors,
		limit:      s.Limit,
	}}
}

type linterSummaryNode struct {
	linter     string
	workDir    string
	violations []Violation
	errors     []string
	limit      int
}

func (n *linterSummaryNode) Pretty() api.Text {
	errs, warns := 0, 0
	for _, v := range n.violations {
		if v.Severity == SeverityWarning {
			warns++
		} else {
			errs++
		}
	}
	if len(n.errors) > 0 || errs > 0 {
		text := fmt.Sprintf("❌ %s (%d errors, %d warnings)", n.linter, errs, warns)
		if summary := firstErrorLine(strings.Join(n.errors, "\n")); summary != "" {
			text += " - " + summary
		}
		return api.Text{Content: text, Style: "text-red-600"}
	}
	if warns == 0 {
		return api.Text{Content: "✅ " + n.linter, Style: "text-green-600"}
	}
	return api.Text{
		Content: fmt.Sprintf("⚠ %s (%d warnings)", n.linter, warns),
		Style:   "text-yellow-600",
	}
}

func (n *linterSummaryNode) GetChildren() []api.TreeNode {
	var children []api.TreeNode
	for _, err := range n.errors {
		children = append(children, &linterErrorNode{message: err})
	}
	if len(n.violations) == 0 {
		return children
	}

	type ruleGroup struct {
		rule       string
		severity   Severity
		violations []Violation
	}
	byRule := map[string]*ruleGroup{}
	var order []string
	for _, v := range n.violations {
		key := v.Rule
		if key == "" {
			key = "(no rule)"
		}
		g, ok := byRule[key]
		if !ok {
			g = &ruleGroup{rule: key, severity: v.Severity}
			byRule[key] = g
			order = append(order, key)
		}
		g.violations = append(g.violations, v)
	}
	sort.Slice(order, func(i, j int) bool {
		left, right := byRule[order[i]], byRule[order[j]]
		if (left.severity == SeverityWarning) != (right.severity == SeverityWarning) {
			return left.severity != SeverityWarning // errors first
		}
		if len(left.violations) != len(right.violations) {
			return len(left.violations) > len(right.violations)
		}
		return left.rule < right.rule
	})

	for _, key := range order {
		g := byRule[key]
		children = append(children, &ruleSummaryNode{
			rule:       g.rule,
			severity:   g.severity,
			workDir:    n.workDir,
			violations: g.violations,
			limit:      n.limit,
		})
	}
	return children
}

type linterErrorNode struct {
	message string
}

func (n *linterErrorNode) Pretty() api.Text {
	return api.Text{Content: strings.TrimRight(n.message, "\n"), Style: "text-red-500"}
}

func (n *linterErrorNode) GetChildren() []api.TreeNode { return nil }

type ruleSummaryNode struct {
	rule       string
	severity   Severity
	workDir    string
	violations []Violation
	limit      int
}

func (n *ruleSummaryNode) Pretty() api.Text {
	label, style := "error", "text-red-500"
	if n.severity == SeverityWarning {
		label, style = "warning", "text-yellow-600"
	}
	text := fmt.Sprintf("[%s] %s (%d)", label, n.rule, len(n.violations))
	if msg := firstMessage(n.violations); msg != "" {
		text += " - " + msg
	}
	return api.Text{Content: text, Style: style}
}

func (n *ruleSummaryNode) GetChildren() []api.TreeNode {
	type fileBucket struct {
		file  string
		first Violation
		count int
	}
	byFile := map[string]*fileBucket{}
	var order []string
	for _, v := range n.violations {
		file := v.File
		if file == "" {
			file = "(unknown file)"
		}
		b, ok := byFile[file]
		if !ok {
			b = &fileBucket{file: file, first: v}
			byFile[file] = b
			order = append(order, file)
		}
		b.count++
	}
	sort.Strings(order)

	limit := min(n.limit, len(order))
	children := make([]api.TreeNode, 0, limit+1)
	for i := range limit {
		b := byFile[order[i]]
		children = append(children, &locationSummaryNode{
			workDir:   n.workDir,
			violation: b.first,
			count:     b.count,
		})
	}
	if remaining := len(order) - limit; remaining > 0 {
		children = append(children, &moreLocationsNode{remaining: remaining})
	}
	return children
}

type locationSummaryNode struct {
	workDir   string
	violation Violation
	count     int
}

func (n *locationSummaryNode) Pretty() api.Text {
	file := n.violation.File
	if file == "" {
		file = "(unknown file)"
	}
	if n.workDir != "" && filepath.IsAbs(file) {
		if rel, err := filepath.Rel(n.workDir, file); err == nil {
			file = rel
		}
	}
	if n.count > 1 {
		return api.Text{
			Content: fmt.Sprintf("📄 %s (%d)", file, n.count),
			Style:   "text-muted",
		}
	}
	loc := file
	if n.violation.Line > 0 {
		loc = fmt.Sprintf("%s:%d", file, n.violation.Line)
		if n.violation.Column > 0 {
			loc = fmt.Sprintf("%s:%d", loc, n.violation.Column)
		}
	}
	return api.Text{Content: "📄 " + loc, Style: "text-muted"}
}

func (n *locationSummaryNode) GetChildren() []api.TreeNode { return nil }

type moreLocationsNode struct {
	remaining int
}

func (n *moreLocationsNode) Pretty() api.Text {
	return api.Text{Content: fmt.Sprintf("... %d more", n.remaining), Style: "text-muted"}
}

func (n *moreLocationsNode) GetChildren() []api.TreeNode { return nil }

func firstMessage(vs []Violation) string {
	for _, v := range vs {
		if strings.TrimSpace(v.Message) != "" {
			return v.Message
		}
	}
	return ""
}

func firstErrorLine(msg string) string {
	for _, line := range strings.Split(msg, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
