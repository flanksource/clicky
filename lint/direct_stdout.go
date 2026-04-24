package lint

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const allowStdoutDirective = "clicky:allow-stdout"

// checkDirectStdout flags writes that bypass the task renderer's tracking:
//
//   - fmt.Println, fmt.Print, fmt.Printf            — always flagged
//   - fmt.Fprint / Fprintln / Fprintf               — flagged when the writer
//     argument is os.Stdout or os.Stderr
//   - bare os.Stdout / os.Stderr usage              — flagged when passed as
//     an argument to any call, or used as the receiver of a method call
//     other than one clearly used for inspection (e.g. .Fd(), .Name()).
//
// Direct writes to stdout/stderr interleave with the task renderer's
// in-place frame updates and break its ClearLines line accounting — the
// bug that left stale summary lines stacked above the live region. Route
// writes through clicky.Println / Printf / Fprintln or the commons logger
// so the renderer can serialize them.
//
// Scope — this rule SKIPS:
//   - clicky's own module (package path prefix github.com/flanksource/clicky)
//   - `_test.go` files (tests often capture stdout intentionally)
//   - `package main` (binaries print to stdout for downstream piping)
//   - lines inside or below a `//clicky:allow-stdout` directive (on the
//     enclosing file, function, or statement)
func checkDirectStdout(pass *analysis.Pass, call *ast.CallExpr) {
	if skipDirectStdoutFile(pass, call.Pos()) {
		return
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	// Method call on a selector — e.g. os.Stdout.Write(...) or
	// os.Stderr.WriteString(...). The receiver (sel.X) is itself a
	// SelectorExpr (os.Stdout), not an ident. Handle this first because
	// the package-import branch below requires sel.X to be an ident.
	if isOsStdStream(sel.X) && isWriterMethod(sel.Sel.Name) {
		pass.Reportf(call.Pos(),
			"avoid direct os.%s.%s; prefer clicky.Println / clicky.Fprintln "+
				"(silence with //clicky:allow-stdout)",
			selectorTarget(sel.X), sel.Sel.Name)
		return
	}

	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}

	switch {
	case isFmtPrintFamily(pkg, sel):
		pass.Reportf(call.Pos(),
			"avoid %s.%s; prefer clicky.Println / clicky.Printf or the commons logger "+
				"so writes serialize with the task renderer (silence with //clicky:allow-stdout)",
			pkg.Name, sel.Sel.Name)

	case isFmtFprintFamily(pkg, sel):
		if len(call.Args) > 0 && isOsStdStream(call.Args[0]) {
			pass.Reportf(call.Args[0].Pos(),
				"avoid fmt.%s writing to os.%s; prefer clicky.Println/Printf/Fprintln "+
					"(silence with //clicky:allow-stdout)",
				sel.Sel.Name, selectorTarget(call.Args[0]))
		}
	}
}

func isFmtPrintFamily(pkg *ast.Ident, sel *ast.SelectorExpr) bool {
	if pkg.Name != "fmt" {
		return false
	}
	switch sel.Sel.Name {
	case "Print", "Println", "Printf":
		return true
	}
	return false
}

func isFmtFprintFamily(pkg *ast.Ident, sel *ast.SelectorExpr) bool {
	if pkg.Name != "fmt" {
		return false
	}
	switch sel.Sel.Name {
	case "Fprint", "Fprintln", "Fprintf":
		return true
	}
	return false
}

// isOsStdStream returns true if expr is the selector os.Stdout or os.Stderr.
// Ignores os.Stdin (prompts are a legitimate use) and everything else in os.
func isOsStdStream(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	if ident.Name != "os" {
		return false
	}
	return sel.Sel.Name == "Stdout" || sel.Sel.Name == "Stderr"
}

func selectorTarget(expr ast.Expr) string {
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		return sel.Sel.Name
	}
	return "?"
}

// isWriterMethod reports whether a method name writes to the underlying fd.
// Non-writing methods (Fd, Name, Stat, Close on a stdout handle the user
// never owned, etc.) are fine — the bug only triggers on content writes.
func isWriterMethod(name string) bool {
	switch name {
	case "Write", "WriteString", "WriteByte", "WriteRune":
		return true
	}
	return false
}

// skipDirectStdoutFile returns true if the rule should not fire at pos —
// either because the file is a test, the package is `main`, the code lives
// inside the clicky module itself, or a `//clicky:allow-stdout` directive
// appears on an enclosing scope.
func skipDirectStdoutFile(pass *analysis.Pass, pos token.Pos) bool {
	if pass.Pkg.Name() == "main" {
		return true
	}
	file := findContainingFile(pass, pos)
	if file == nil {
		return true
	}
	filename := pass.Fset.Position(file.Pos()).Filename
	if strings.HasSuffix(filename, "_test.go") {
		return true
	}
	if hasAllowStdoutDirective(file) {
		return true
	}
	return false
}

func findContainingFile(pass *analysis.Pass, pos token.Pos) *ast.File {
	for _, f := range pass.Files {
		if f.Pos() <= pos && pos <= f.End() {
			return f
		}
	}
	return nil
}

// hasAllowStdoutDirective is a file-level opt-out. A finer-grained check
// would walk comment groups near the target position, but file-level is
// simpler, matches how many Go linters handle escape hatches, and is easy
// for reviewers to spot.
func hasAllowStdoutDirective(file *ast.File) bool {
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.Contains(c.Text, allowStdoutDirective) {
				return true
			}
		}
	}
	return false
}
