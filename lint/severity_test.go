package lint

import "testing"

func TestSeverityParsedFromCategoryDefaultsToError(t *testing.T) {
	cases := map[string]Severity{
		"warning:manual-cobra-command": SeverityWarning,
		"error:manual-cobra-command":   SeverityError,
		string(SeverityWarning):        SeverityWarning,
		string(SeverityError):          SeverityError,
		"":                             SeverityError,
		"unknown":                      SeverityError,
	}
	for category, want := range cases {
		if got, _ := parseCategory(category); got != want {
			t.Errorf("parseCategory(%q) = %q, want %q", category, got, want)
		}
	}
}

func TestResultSeverityCountsAndExit(t *testing.T) {
	r := &Result{
		Violations: []Violation{
			{Severity: SeverityError, Rule: "manual cobra"},
			{Severity: SeverityWarning, Rule: "pretty preference"},
			{Severity: SeverityWarning, Rule: "missing table provider"},
		},
	}
	if got := r.ErrorCount(); got != 1 {
		t.Errorf("ErrorCount() = %d, want 1", got)
	}
	if got := r.WarningCount(); got != 2 {
		t.Errorf("WarningCount() = %d, want 2", got)
	}
	if !r.HasErrors() {
		t.Error("HasErrors() = false, want true (one error-severity violation)")
	}
}

func TestResultWarningsOnlyDoNotFail(t *testing.T) {
	r := &Result{
		Violations: []Violation{
			{Severity: SeverityWarning, Rule: "pretty preference"},
		},
	}
	if r.HasErrors() {
		t.Error("HasErrors() = true, want false (warnings must not fail the run)")
	}
	if !r.HasIssues() {
		t.Error("HasIssues() = false, want true (a warning is still an issue to display)")
	}
}

func TestResultLoadErrorsFail(t *testing.T) {
	r := &Result{Errors: []string{"package load failed"}}
	if !r.HasErrors() {
		t.Error("HasErrors() = false, want true (load errors fail the run)")
	}
}

func TestDuplicateFindingsFromTheTestVariantCollapse(t *testing.T) {
	// A package and its test variant share a path and both carry the production
	// files, so the same finding arrives twice.
	finding := Violation{Package: "example/pkg", File: "a.go", Line: 10, Column: 2, Rule: "direct-stdout", Severity: SeverityError}
	other := Violation{Package: "example/pkg", File: "a.go", Line: 11, Column: 2, Rule: "direct-stdout", Severity: SeverityError}
	violations := []Violation{finding, finding, other}

	sortViolations(violations)
	got := dedupeViolations(violations)

	if len(got) != 2 {
		t.Fatalf("expected the duplicate to collapse, got %d violations: %+v", len(got), got)
	}
}

func TestDistinctFindingsAtTheSamePositionSurvive(t *testing.T) {
	// Two rules can legitimately fire at one position; only exact repeats go.
	position := Violation{Package: "example/pkg", File: "a.go", Line: 10, Column: 2, Severity: SeverityWarning}
	first, second := position, position
	first.Rule, second.Rule = "direct-http-handler", "servemux-parameter"
	violations := []Violation{first, second}

	sortViolations(violations)
	if got := dedupeViolations(violations); len(got) != 2 {
		t.Fatalf("two different rules at one position must both be reported, got %+v", got)
	}
}
