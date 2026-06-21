package lint

import (
	"fmt"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// Severity classifies a clickylint diagnostic. Errors fail the lint run (non-zero
// exit); warnings are advisory and do not affect the exit status.
type Severity string

const (
	// SeverityError marks structural violations that bypass clicky's generated
	// surfaces — manual cobra commands and direct HTTP handlers instead of
	// registered entities, or writes that corrupt the task renderer.
	SeverityError Severity = "error"
	// SeverityWarning marks advisory style preferences — how Pretty()/render
	// builders are written, and entities missing a TableProvider.
	SeverityWarning Severity = "warning"
)

// report emits a diagnostic carrying its severity in Diagnostic.Category, which
// the runner reads back into Violation.Severity. go/analysis has no native
// severity, so Category is the carrier.
func report(pass *analysis.Pass, sev Severity, pos token.Pos, format string, args ...any) {
	pass.Report(analysis.Diagnostic{
		Pos:      pos,
		Category: string(sev),
		Message:  fmt.Sprintf(format, args...),
	})
}
