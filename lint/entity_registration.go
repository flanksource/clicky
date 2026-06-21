package lint

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

const (
	cobraPkgPath    = "github.com/spf13/cobra"
	netHTTPPkgPath  = "net/http"
	entityPkgPath   = clickyModulePrefix + "/entity"
	tableProvidName = "TableProvider"
)

// checkManualCobraCommand (error) flags hand-rolled cobra commands that carry a
// Run/RunE handler. Such leaf commands should be defined as registered entities
// (clicky.NewEntity(...).Register() + GenerateCLI) or via entity.AddCommand, so
// they participate in the generated CLI/REST/MCP surfaces. Bare grouping
// commands (no Run/RunE) are structural and left alone.
func checkManualCobraCommand(pass *analysis.Pass, lit *ast.CompositeLit) {
	if !isNamedType(pass.TypesInfo.TypeOf(lit), cobraPkgPath, "Command") {
		return
	}
	if !hasRunField(lit) {
		return
	}
	report(pass, SeverityError, lit.Pos(),
		"avoid manual cobra.Command with Run/RunE; register the operation via "+
			"clicky.NewEntity(...).Register() + GenerateCLI, or entity.AddCommand")
}

func hasRunField(lit *ast.CompositeLit) bool {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if ident, ok := kv.Key.(*ast.Ident); ok && (ident.Name == "Run" || ident.Name == "RunE") {
			return true
		}
	}
	return false
}

// checkHTTPHandlerRegistration (error) flags direct route registration on a
// net/http mux (http.HandleFunc / http.Handle / (*http.ServeMux).HandleFunc /
// .Handle). Routes in a clicky app should come from registered entities served
// through the rpc layer, not from raw handlers that collide with the
// auto-routed /api/v1/* namespace.
func checkHTTPHandlerRegistration(pass *analysis.Pass, call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	name := sel.Sel.Name
	if name != "HandleFunc" && name != "Handle" {
		return
	}
	fn, ok := pass.TypesInfo.ObjectOf(sel.Sel).(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != netHTTPPkgPath {
		return
	}
	report(pass, SeverityError, call.Pos(),
		"avoid registering net/http handlers directly; expose data via "+
			"clicky.NewEntity(...).Register() and serve it through the rpc layer")
}

// checkEntityTableProvider (warning) flags entities registered via
// NewEntity/RegisterEntity whose item type does not implement api.TableProvider,
// so list output cannot render as a table.
func checkEntityTableProvider(pass *analysis.Pass, call *ast.CallExpr) {
	ident := funcIdent(call.Fun)
	if ident == nil {
		return
	}
	if !isEntityRegistrationFunc(pass, ident) {
		return
	}
	inst, ok := pass.TypesInfo.Instances[ident]
	if !ok || inst.TypeArgs == nil || inst.TypeArgs.Len() == 0 {
		return
	}
	item := inst.TypeArgs.At(0)
	if _, ok := item.(*types.TypeParam); ok {
		return
	}
	iface := lookupClickyInterface(pass, tableProvidName)
	if iface == nil {
		return
	}
	if types.Implements(item, iface) || types.Implements(types.NewPointer(item), iface) {
		return
	}
	report(pass, SeverityWarning, call.Pos(),
		"entity does not implement api.TableProvider; add Columns()/Row() to %s for table rendering",
		shortTypeName(item))
}

func isEntityRegistrationFunc(pass *analysis.Pass, ident *ast.Ident) bool {
	fn, ok := pass.TypesInfo.ObjectOf(ident).(*types.Func)
	if !ok || fn.Pkg() == nil {
		return false
	}
	path := fn.Pkg().Path()
	if path != clickyModulePrefix && path != entityPkgPath {
		return false
	}
	return fn.Name() == "NewEntity" || fn.Name() == "RegisterEntity"
}

func funcIdent(fun ast.Expr) *ast.Ident {
	switch f := fun.(type) {
	case *ast.Ident:
		return f
	case *ast.SelectorExpr:
		return f.Sel
	case *ast.IndexExpr:
		return funcIdent(f.X)
	case *ast.IndexListExpr:
		return funcIdent(f.X)
	}
	return nil
}

func shortTypeName(t types.Type) string {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	if named, ok := t.(*types.Named); ok {
		return named.Obj().Name()
	}
	return t.String()
}

func isNamedType(t types.Type, pkgPath, name string) bool {
	if t == nil {
		return false
	}
	if ptr, ok := t.(*types.Pointer); ok {
		return isNamedType(ptr.Elem(), pkgPath, name)
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Name() == name && obj.Pkg() != nil && obj.Pkg().Path() == pkgPath
}

// lookupClickyInterface resolves an interface type from the clicky api package,
// searching transitively so it works even when api is not a direct import of
// the analyzed package.
func lookupClickyInterface(pass *analysis.Pass, name string) *types.Interface {
	apiPkg := findImportedPackage(pass, clickyAPIPath)
	if apiPkg == nil {
		return nil
	}
	obj := apiPkg.Scope().Lookup(name)
	if obj == nil {
		return nil
	}
	named, ok := obj.Type().(*types.Named)
	if !ok {
		return nil
	}
	iface, _ := named.Underlying().(*types.Interface)
	return iface
}

func findImportedPackage(pass *analysis.Pass, path string) *types.Package {
	seen := map[string]bool{}
	var visit func(p *types.Package) *types.Package
	visit = func(p *types.Package) *types.Package {
		if p == nil || seen[p.Path()] {
			return nil
		}
		seen[p.Path()] = true
		if p.Path() == path {
			return p
		}
		for _, imp := range p.Imports() {
			if found := visit(imp); found != nil {
				return found
			}
		}
		return nil
	}
	for _, imp := range pass.Pkg.Imports() {
		if found := visit(imp); found != nil {
			return found
		}
	}
	return nil
}
