package main

import (
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var (
	processPublish    = objectRef{Path: internalPrefix + "service/process", Name: "Publish"}
	busSeams          = map[string]bool{"Publisher": true, "Context": true}
	dispatchPublisher = objectRef{Path: internalPrefix + "dispatch", Name: "Publisher"}
	domainEvent       = objectRef{Path: internalPrefix + "domain", Name: "Event"}
)

func silentPackages() []dir { return append([]dir{"command", "query"}, engineServices...) }

var publishAnalyzer = &analysis.Analyzer{
	Name: "publish",
	Doc:  "process.Publish is called from service/events only, the flow seam's Publish from internal/flow only, dispatch.Publisher's from dispatch only, and the engine's commands, queries and services publish nothing",
	Run:  runPublish,
}

func runPublish(pass *analysis.Pass) (any, error) {
	pkg := pass.Pkg.Path()
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok && silent(pkg) && !publishes(pass, call.Fun) && handsOnEvent(pass, call) {
				pass.Reportf(call.Pos(), "%s hands a domain.Event to a port: in the engine only dispatch publishes, from the events a command declares at its registration (docs/dev/dispatch.md, publish)", internalPath(pkg))
				return true
			}
			sel, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if silent(pkg) && publishes(pass, sel) {
				pass.Reportf(sel.Pos(), "%s publishes: in the engine only dispatch publishes, what a command reports (docs/dev/dispatch.md, publish)", internalPath(pkg))
				return true
			}
			if refers(pass.TypesInfo.Uses[sel.Sel], processPublish) && !dir("service/events").holds(pkg) && !dir("service/process").holds(pkg) {
				pass.Reportf(sel.Pos(), "process.Publish is called from %s: only service/events publishes, so every event goes out stamped and versioned", internalPath(pkg))
				return true
			}
			if receivedBy(pass, sel, dispatchPublisher) && !dir("dispatch").holds(pkg) {
				pass.Reportf(sel.Pos(), "dispatch.Publisher.Publish is called from %s: only dispatch publishes, what a command reports (docs/dev/dispatch.md, publish)", internalPath(pkg))
				return true
			}
			if seam := flowSeamOf(pass, sel); seam != "" && !dir("flow").holds(pkg) {
				pass.Reportf(sel.Pos(), "flow.%s.Publish is called from %s/: only internal/flow publishes — a change no flow made is a change no surface can replay", seam, layerOfPackage(pkg))
			}
			return true
		})
	}
	return nil, nil
}

func flowSeamOf(pass *analysis.Pass, sel *ast.SelectorExpr) string {
	selection, ok := pass.TypesInfo.Selections[sel]
	if !ok || selection.Obj().Name() != "Publish" {
		return ""
	}
	recv := selection.Recv()
	if ptr, ok := recv.(*types.Pointer); ok {
		recv = ptr.Elem()
	}
	named, ok := recv.(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != internalPrefix+"flow" || !busSeams[named.Obj().Name()] {
		return ""
	}
	return named.Obj().Name()
}

func silent(pkgPath string) bool {
	return slices.ContainsFunc(silentPackages(), func(d dir) bool { return d.holds(pkgPath) })
}

func publishes(pass *analysis.Pass, expr ast.Expr) bool {
	fn, ok := objectOf(pass.TypesInfo, ast.Unparen(expr)).(*types.Func)
	return ok && fn.Name() == "Publish"
}

func receivedBy(pass *analysis.Pass, sel *ast.SelectorExpr, ref objectRef) bool {
	selection, ok := pass.TypesInfo.Selections[sel]
	if !ok || selection.Obj().Name() != "Publish" {
		return false
	}
	recv := selection.Recv()
	if ptr, ok := recv.(*types.Pointer); ok {
		recv = ptr.Elem()
	}
	named, ok := recv.(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == ref.Path && named.Obj().Name() == ref.Name
}

func handsOnEvent(pass *analysis.Pass, call *ast.CallExpr) bool {
	sig, ok := pass.TypesInfo.TypeOf(call.Fun).(*types.Signature)
	if !ok || !dynamicCall(pass, call) {
		return false
	}
	for param := range sig.Params().Variables() {
		if isEvent(param.Type()) {
			return true
		}
	}
	return false
}

func dynamicCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
		if selection, ok := pass.TypesInfo.Selections[sel]; ok && selection.Kind() == types.MethodVal {
			return types.IsInterface(selection.Recv())
		}
	}
	tv, ok := pass.TypesInfo.Types[call.Fun]
	if !ok || tv.IsType() || tv.IsBuiltin() {
		return false
	}
	_, static := objectOf(pass.TypesInfo, ast.Unparen(call.Fun)).(*types.Func)
	return !static
}

func isEvent(typ types.Type) bool {
	if slice, ok := typ.(*types.Slice); ok {
		typ = slice.Elem()
	}
	if pointer, ok := typ.(*types.Pointer); ok {
		typ = pointer.Elem()
	}
	named, ok := typ.(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == domainEvent.Path && named.Obj().Name() == domainEvent.Name
}
