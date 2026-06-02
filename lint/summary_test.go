package lint

import (
	"strings"
	"testing"
)

func testViolation(file string, line int, rule, msg string) Violation {
	return Violation{
		File:    file,
		Line:    line,
		Column:  7,
		Rule:    rule,
		Message: msg,
	}
}

func TestSummaryViewGroupsByRuleAndLocation(t *testing.T) {
	result := &Result{
		Linter:  "clickylint",
		WorkDir: "/repo",
		Violations: []Violation{
			testViolation("/repo/a.go", 10, "avoid direct api.Text struct literal", "avoid direct api.Text struct literal; use api.Text{}.Append(...)"),
			testViolation("/repo/b.go", 12, "avoid direct api.Text struct literal", "avoid direct api.Text struct literal; use api.Text{}.Append(...)"),
			testViolation("/repo/c.go", 14, "avoid .ANSI() inside clicky render builders", "avoid .ANSI() inside clicky render builders; return api.Text"),
		},
	}

	view := NewSummaryView(result, 5)
	children := view.GetChildren()
	if len(children) != 1 {
		t.Fatalf("expected one linter node, got %d", len(children))
	}
	linter := children[0].(*linterSummaryNode)
	rules := linter.GetChildren()
	if len(rules) != 2 {
		t.Fatalf("expected two rule nodes, got %d", len(rules))
	}
	first := rules[0].(*ruleSummaryNode)
	if first.rule != "avoid direct api.Text struct literal" {
		t.Fatalf("expected most frequent rule first, got %q", first.rule)
	}
	if len(first.violations) != 2 {
		t.Fatalf("expected two violations in first group, got %d", len(first.violations))
	}

	locations := first.GetChildren()
	if len(locations) != 2 {
		t.Fatalf("expected two file locations, got %d", len(locations))
	}
	if got := locations[0].Pretty().String(); !strings.Contains(got, "a.go:10:7") {
		t.Fatalf("expected relative file location in first child, got %q", got)
	}
}

func TestSummaryViewTruncatesLocationsAtLimit(t *testing.T) {
	result := &Result{Linter: "clickylint"}
	for i, file := range []string{"a.go", "b.go", "c.go"} {
		result.Violations = append(result.Violations, testViolation(file, i+1, "same-rule", "same-rule; details"))
	}

	rule := NewSummaryView(result, 2).GetChildren()[0].GetChildren()[0]
	children := rule.GetChildren()
	if len(children) != 3 {
		t.Fatalf("expected 2 locations and a trailer, got %d", len(children))
	}
	trailer, ok := children[2].(*moreLocationsNode)
	if !ok {
		t.Fatalf("expected trailer node, got %T", children[2])
	}
	if trailer.remaining != 1 {
		t.Fatalf("expected 1 remaining location, got %d", trailer.remaining)
	}
}

func TestSummaryViewSurfacesErrors(t *testing.T) {
	result := &Result{
		Linter: "clickylint",
		Errors: []string{"package load failed:\nmissing module"},
	}

	view := NewSummaryView(result, 5)
	if got := view.Pretty().String(); !strings.Contains(got, "1 errors") {
		t.Fatalf("expected root error count, got %q", got)
	}
	linter := view.GetChildren()[0].(*linterSummaryNode)
	if got := linter.Pretty().String(); !strings.Contains(got, "package load failed") {
		t.Fatalf("expected linter error preview, got %q", got)
	}
	if children := linter.GetChildren(); len(children) != 1 {
		t.Fatalf("expected one error child, got %d", len(children))
	}
}

func TestRuleForMessageUsesFirstClause(t *testing.T) {
	got := RuleForMessage("avoid .ANSI() inside clicky render builders; return api.Text")
	if got != "avoid .ANSI() inside clicky render builders" {
		t.Fatalf("unexpected rule: %q", got)
	}
}
