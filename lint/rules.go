// Rule is the stable name of one clickylint check, so a report can be
// suppressed, filtered or re-levelled by name instead of by its message text.
package lint

import "strings"

// Rule names a check and carries the severity it reports at by default.
// The ID is part of clickylint's contract: it appears in JSON output and is
// what a //clicky:allow directive and a severity override both address. Message
// wording is free to change; an ID is not.
type Rule struct {
	ID       string
	Severity Severity
}

// The rule catalogue. Adding a check means adding its rule here first, so every
// diagnostic is addressable from the moment it can be emitted.
var (
	RuleTextStructLiteral   = Rule{ID: "text-struct-literal", Severity: SeverityWarning}
	RuleHelperBackedLiteral = Rule{ID: "helper-backed-literal", Severity: SeverityWarning}
	RuleContentConcat       = Rule{ID: "content-concatenation", Severity: SeverityWarning}
	RuleContentSprintf      = Rule{ID: "content-sprintf", Severity: SeverityWarning}
	RuleChildrenLiteral     = Rule{ID: "children-literal", Severity: SeverityWarning}
	RuleTextReturnType      = Rule{ID: "text-return-type", Severity: SeverityWarning}
	RuleRenderInPretty      = Rule{ID: "render-in-pretty", Severity: SeverityWarning}
	RuleDirectStdout        = Rule{ID: "direct-stdout", Severity: SeverityError}
	RuleManualCobraCommand  = Rule{ID: "manual-cobra-command", Severity: SeverityError}
	RuleDirectHTTPHandler   = Rule{ID: "direct-http-handler", Severity: SeverityWarning}
	RuleServeMuxParameter   = Rule{ID: "servemux-parameter", Severity: SeverityWarning}
	RuleEntityTableProvider = Rule{ID: "entity-table-provider", Severity: SeverityWarning}
)

// rules is every rule the analyzer can emit, for resolving an ID back to a rule
// when a caller overrides a severity or writes a suppression directive.
var rules = []Rule{
	RuleTextStructLiteral, RuleHelperBackedLiteral, RuleContentConcat, RuleContentSprintf,
	RuleChildrenLiteral, RuleTextReturnType, RuleRenderInPretty, RuleDirectStdout,
	RuleManualCobraCommand, RuleDirectHTTPHandler, RuleServeMuxParameter, RuleEntityTableProvider,
}

// RuleIDs lists every rule clickylint can report, so a caller can validate a
// severity override or a suppression directive against the real set.
func RuleIDs() []string {
	ids := make([]string, 0, len(rules))
	for _, rule := range rules {
		ids = append(ids, rule.ID)
	}
	return ids
}

// category packs a rule's severity and ID into the one string field
// go/analysis gives a diagnostic. The runner unpacks it again; nothing else
// should read it.
func (r Rule) category() string { return string(r.Severity) + ":" + r.ID }

// parseCategory recovers what category packed. A category that predates this
// encoding, or that some other driver produced, is reported as an error with no
// rule ID rather than silently demoted to a warning.
func parseCategory(category string) (Severity, string) {
	severity, id, found := strings.Cut(category, ":")
	if !found {
		if Severity(category) == SeverityWarning {
			return SeverityWarning, ""
		}
		return SeverityError, ""
	}
	if Severity(severity) == SeverityWarning {
		return SeverityWarning, id
	}
	return SeverityError, id
}
