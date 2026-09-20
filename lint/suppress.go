// Suppression of individual clickylint findings through a source directive, so
// a deliberate exception is recorded where it is made rather than argued for in
// a review thread.
package lint

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// allowDirective opts one finding out of one rule:
//
//	//clicky:allow <rule-id> <reason>
//
// It applies to the line it sits on and to the statement directly below it, so
// a file registering a hundred routes can excuse the four that genuinely cannot
// move without excusing the rest. Placed above the package clause it covers the
// whole file, for the case where every occurrence is deliberate. The reason is
// not parsed — it is there for whoever reads the line next, and a directive
// without one is still honoured.
const allowDirective = "//clicky:allow"

// allowances is the set of rules excused at each line of a file, plus which
// lines are comments, so a directive can be traced upward from the finding it
// covers through a contiguous comment block.
type allowances struct {
	rules    map[int]map[string]bool
	comments map[int]bool
}

// fileAllowances reads every directive in a file once.
func fileAllowances(fset *token.FileSet, file *ast.File) allowances {
	found := allowances{rules: map[int]map[string]bool{}, comments: map[int]bool{}}
	for _, group := range file.Comments {
		for _, comment := range group.List {
			line := fset.Position(comment.Pos()).Line
			found.comments[line] = true
			ids := directiveRules(comment.Text)
			if len(ids) == 0 {
				continue
			}
			if found.rules[line] == nil {
				found.rules[line] = map[string]bool{}
			}
			for _, id := range ids {
				found.rules[line][id] = true
			}
		}
	}
	return found
}

// directiveRules returns the rule IDs a comment excuses, or nothing when it is
// not a directive. Several IDs may be listed before the reason:
//
//	//clicky:allow direct-http-handler servemux-parameter reverse proxy
//
// Reading stops at the first word that is not a known rule ID, which is where
// the reason begins.
func directiveRules(text string) []string {
	body, found := strings.CutPrefix(strings.TrimSpace(strings.TrimPrefix(text, "/*")), allowDirective)
	if !found {
		return nil
	}
	// Require a separator so //clicky:allow-something is not read as a bare
	// directive excusing everything.
	if body != "" && !strings.HasPrefix(body, " ") && !strings.HasPrefix(body, "\t") {
		return nil
	}
	known := map[string]bool{}
	for _, id := range RuleIDs() {
		known[id] = true
	}
	var ids []string
	for _, field := range strings.Fields(body) {
		if !known[field] {
			break
		}
		ids = append(ids, field)
	}
	return ids
}

// suppressed reports whether rule is excused at pos: by a directive trailing the
// same line, or by one in the contiguous comment block immediately above it.
func suppressed(pass *analysis.Pass, pos token.Pos, rule Rule) bool {
	file := enclosingFile(pass, pos)
	if file == nil {
		return false
	}
	found := fileAllowances(pass.Fset, file)
	packageLine := pass.Fset.Position(file.Package).Line
	for above := 1; above < packageLine; above++ {
		if found.rules[above][rule.ID] {
			return true
		}
	}
	line := pass.Fset.Position(pos).Line
	if found.rules[line][rule.ID] {
		return true
	}
	for above := line - 1; above > 0 && found.comments[above]; above-- {
		if found.rules[above][rule.ID] {
			return true
		}
	}
	return false
}

// enclosingFile returns the parsed file containing pos.
func enclosingFile(pass *analysis.Pass, pos token.Pos) *ast.File {
	for _, file := range pass.Files {
		if file.Pos() <= pos && pos <= file.End() {
			return file
		}
	}
	return nil
}
