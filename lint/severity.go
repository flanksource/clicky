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
	// SeverityError marks violations that fail the run: writes that corrupt the
	// task renderer, and manual cobra commands that bypass the generated CLI,
	// REST and MCP surfaces entirely.
	SeverityError Severity = "error"
	// SeverityWarning marks advice — how Pretty()/render builders are written,
	// entities missing a TableProvider, and routes registered outside the
	// router. A codebase adopting a rule re-levels it with RunOptions.Severity
	// rather than each rule carrying a second, configurable default.
	SeverityWarning Severity = "warning"
)

// report emits a diagnostic for rule unless the source excuses it there.
// go/analysis has no native severity or rule field, so both travel in
// Diagnostic.Category, which the runner unpacks into Violation.
func report(pass *analysis.Pass, rule Rule, pos token.Pos, format string, args ...any) {
	if suppressed(pass, pos, rule) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      pos,
		Category: rule.category(),
		Message:  fmt.Sprintf(format, args...),
	})
}
