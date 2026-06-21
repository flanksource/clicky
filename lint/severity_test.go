package lint

import "testing"

func TestSeverityFromCategoryDefaultsToError(t *testing.T) {
	cases := map[string]Severity{
		string(SeverityWarning): SeverityWarning,
		string(SeverityError):   SeverityError,
		"":                      SeverityError,
		"unknown":               SeverityError,
	}
	for category, want := range cases {
		if got := severityFromCategory(category); got != want {
			t.Errorf("severityFromCategory(%q) = %q, want %q", category, got, want)
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
