package lint

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var prettyBuilderNames = map[string]bool{
	"Pretty":      true,
	"PrettyFull":  true,
	"PrettyShort": true,
	"PrettyRow":   true,
}

var renderExtractionMethods = map[string]bool{
	"ANSI":          true,
	"HTML":          true,
	"Markdown":      true,
	"MarkdownSlack": true,
	"StaticHTML":    true,
	"CompactHTML":   true,
	"AsANSI":        true,
	"AsHTML":        true,
	"AsMarkdown":    true,
	"String":        true,
}

func checkRenderBuilderRenderCalls(pass *analysis.Pass, fn *ast.FuncDecl) {
	if fn.Body == nil || !isRenderBuilderFunc(pass, fn) {
		return
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !renderExtractionMethods[sel.Sel.Name] {
			return true
		}
		if !isClickyRenderType(pass, pass.TypesInfo.TypeOf(sel.X)) {
			return true
		}
		pass.Reportf(call.Pos(),
			"avoid .%s() inside clicky render builders; return api.Text/api.Textable and let the formatter render it",
			sel.Sel.Name)
		return true
	})
}

func isRenderBuilderFunc(pass *analysis.Pass, fn *ast.FuncDecl) bool {
	if prettyBuilderNames[fn.Name.Name] {
		return true
	}
	if fn.Type.Results == nil {
		return false
	}
	for _, result := range fn.Type.Results.List {
		if isClickyRenderType(pass, pass.TypesInfo.TypeOf(result.Type)) {
			return true
		}
	}
	return false
}

func isClickyRenderType(pass *analysis.Pass, t types.Type) bool {
	if t == nil {
		return false
	}

	switch tt := t.(type) {
	case *types.Pointer:
		return isClickyRenderType(pass, tt.Elem()) || implementsClickyTextable(pass, tt)
	case *types.Slice:
		return isClickyRenderType(pass, tt.Elem())
	case *types.Array:
		return isClickyRenderType(pass, tt.Elem())
	case *types.Map:
		return isClickyRenderType(pass, tt.Elem())
	case *types.Named:
		if isClickyNamedType(tt,
			"Text",
			"Textable",
			"TextList",
			"TextTable",
			"TextTree",
			"Code",
			"Link",
			"LinkCommand",
			"Button",
			"ButtonGroup",
			"KeyValuePair",
			"DescriptionList",
			"LabelBadge",
			"Heading",
			"Blockquote",
			"Admonition",
			"FootnoteRef",
			"Footnote",
			"Footnotes",
			"Collapsed",
			"Diff",
			"StackTrace",
			"HtmlElement",
			"TypedValue",
			"TypedList",
			"TypedMap",
			"TextMap",
		) {
			return true
		}
		if implementsClickyTextable(pass, tt) || implementsClickyTextable(pass, types.NewPointer(tt)) {
			return true
		}
		return isClickyRenderType(pass, tt.Underlying())
	case *types.Interface:
		return isClickyNamedType(t, "Textable") || implementsClickyTextable(pass, tt)
	default:
		return implementsClickyTextable(pass, t)
	}
}

func implementsClickyTextable(pass *analysis.Pass, t types.Type) bool {
	if t == nil {
		return false
	}
	iface := clickyTextableInterface(pass)
	if iface == nil {
		return false
	}
	return types.Implements(t, iface)
}

func clickyTextableInterface(pass *analysis.Pass) *types.Interface {
	for _, pkg := range pass.Pkg.Imports() {
		if pkg.Path() != clickyAPIPath {
			continue
		}
		obj := pkg.Scope().Lookup("Textable")
		if obj == nil {
			return nil
		}
		named, ok := obj.Type().(*types.Named)
		if !ok {
			return nil
		}
		iface, ok := named.Underlying().(*types.Interface)
		if !ok {
			return nil
		}
		return iface
	}
	return nil
}
