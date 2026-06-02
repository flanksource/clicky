package lint

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const clickyAPIPath = "github.com/flanksource/clicky/api"
const clickyModulePrefix = "github.com/flanksource/clicky"

var helperBackedTypes = map[string]string{
	"Text":            "clicky.Text(...) or api.Text{}.Append(...)",
	"List":            "clicky.List(...)",
	"TextList":        "clicky.TextList(...)",
	"TextTable":       "clicky.Table(...) or PrettyRow/TableMixin",
	"TextTree":        "clicky.Tree(...) or TreeNode/TreeMixin",
	"Code":            "clicky.CodeBlock(...)",
	"Button":          "clicky.Button(...)",
	"ButtonGroup":     "clicky.ButtonGroup(...)",
	"KeyValuePair":    "clicky.KeyValue(...)",
	"DescriptionList": "clicky.Map(...)",
	"LabelBadge":      "clicky.LabelBadge(...)",
	"Admonition":      "clicky.Admonition(...)",
	"Collapsed":       "clicky.Collapsed(...)",
	"Diff":            "clicky.Diff(...)",
	"StackTrace":      "clicky.StackTrace(...)",
	"HtmlElement":     "clicky.HTMLElement(...)",
}

var Analyzer = &analysis.Analyzer{
	Name:     "clickylint",
	Doc:      "detects bad usage patterns of the clicky text API and render builders",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	if isClickyModule(pass) {
		return nil, nil
	}

	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.CompositeLit)(nil),
		(*ast.FuncDecl)(nil),
		(*ast.CallExpr)(nil),
	}

	ins.Preorder(nodeFilter, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.CompositeLit:
			checkCompositeLiteral(pass, node)
		case *ast.FuncDecl:
			checkFuncReturnType(pass, node)
			checkRenderBuilderRenderCalls(pass, node)
		case *ast.CallExpr:
			checkDirectStdout(pass, node)
		}
	})

	return nil, nil
}

func isClickyModule(pass *analysis.Pass) bool {
	return strings.HasPrefix(pass.Pkg.Path(), clickyModulePrefix)
}

func isClickyTextType(pass *analysis.Pass, expr ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(expr)
	if t == nil {
		return false
	}
	return isClickyTextTypesType(t)
}

func isClickyTextTypesType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		if ptr, ok := t.(*types.Pointer); ok {
			return isClickyTextTypesType(ptr.Elem())
		}
		return false
	}
	obj := named.Obj()
	return obj.Name() == "Text" && obj.Pkg() != nil && obj.Pkg().Path() == clickyAPIPath
}

func isClickyNamedType(t types.Type, names ...string) bool {
	typeName, ok := clickyAPITypeName(t)
	if !ok {
		return false
	}
	for _, candidate := range names {
		if typeName == candidate {
			return true
		}
	}
	return false
}

func clickyAPITypeName(t types.Type) (string, bool) {
	named, ok := t.(*types.Named)
	if !ok {
		if ptr, ok := t.(*types.Pointer); ok {
			return clickyAPITypeName(ptr.Elem())
		}
		return "", false
	}
	obj := named.Obj()
	if obj.Pkg() == nil || obj.Pkg().Path() != clickyAPIPath {
		return "", false
	}
	return obj.Name(), true
}

// checkCompositeLiteral checks rules 1 (struct literal), 3 (concat content), 4 (children literal)
func checkCompositeLiteral(pass *analysis.Pass, lit *ast.CompositeLit) {
	typeName, isClickyAPIType := clickyAPITypeName(pass.TypesInfo.TypeOf(lit))
	if !isClickyAPIType {
		return
	}

	if typeName == "Text" {
		if len(lit.Elts) > 0 {
			pass.Reportf(lit.Pos(),
				"avoid direct api.Text struct literal; use clicky.Text(...) or api.Text{}.Append(...)")
		}
	} else if helper, ok := helperBackedTypes[typeName]; ok {
		pass.Reportf(lit.Pos(),
			"avoid direct api.%s struct literal; use %s", typeName, helper)
	}

	if typeName != "Text" {
		return
	}

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		ident, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		switch ident.Name {
		case "Content":
			checkContentField(pass, kv)
		case "Children":
			checkChildrenField(pass, kv)
		}
	}
}

// checkContentField checks rule 3: string concatenation or fmt.Sprintf in Content
func checkContentField(pass *analysis.Pass, kv *ast.KeyValueExpr) {
	switch v := kv.Value.(type) {
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			pass.Reportf(kv.Value.Pos(),
				"avoid string concatenation in Content field; use .Append()/.Appendf()")
		}
	case *ast.CallExpr:
		if isFmtSprintf(v) {
			pass.Reportf(kv.Value.Pos(),
				"avoid fmt.Sprintf in Content field; use .Appendf()")
		}
	}
}

func isFmtSprintf(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "fmt" && sel.Sel.Name == "Sprintf"
}

// checkChildrenField checks rule 4: Children slice literal with api.Text elements
func checkChildrenField(pass *analysis.Pass, kv *ast.KeyValueExpr) {
	sliceLit, ok := kv.Value.(*ast.CompositeLit)
	if !ok {
		return
	}

	for _, elt := range sliceLit.Elts {
		innerLit, ok := elt.(*ast.CompositeLit)
		if !ok {
			continue
		}
		if isClickyTextType(pass, innerLit) {
			pass.Reportf(kv.Pos(),
				"avoid Children slice literal with api.Text elements; use .Add()/.Append() chaining")
			return
		}
	}
}

// checkFuncReturnType checks rule 2: non-Pretty functions returning api.Text
func checkFuncReturnType(pass *analysis.Pass, fn *ast.FuncDecl) {
	if fn.Type.Results == nil {
		return
	}

	name := fn.Name.Name
	if name == "Pretty" || name == "PrettyFull" || name == "PrettyShort" || name == "PrettyRow" {
		return
	}

	for _, result := range fn.Type.Results.List {
		t := pass.TypesInfo.TypeOf(result.Type)
		if t != nil && isClickyTextTypesType(t) {
			pass.Reportf(fn.Name.Pos(),
				"%s returns api.Text; return api.Textable interface or rename to Pretty/PrettyFull/PrettyRow",
				name)
			return
		}
	}
}
