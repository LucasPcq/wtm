package main

import (
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var (
	processPublish = objectRef{Path: internalPrefix + "service/process", Name: "Publish"}
	// busSeams are the flow types whose Publish only a flow may call: an event
	// published from anywhere else reports a change no flow made, which no other
	// surface could replay.
	busSeams = map[string]bool{"Publisher": true, "Context": true}
	// dispatchPublisher is the engine's one way out for an event: dispatch
	// publishes what a command reports, stamped with its operation.
	dispatchPublisher = objectRef{Path: internalPrefix + "dispatch", Name: "Publisher"}
)

// silentPackages publish nothing at all, whatever the bus: a mutator returns
// what it changed and the command reports it.
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

// flowSeamOf names the flow type a Publish call goes through, empty when it is
// not one of the bus seams.
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

func publishes(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	return ok && fn.Name() == "Publish"
}

// receivedBy tells a Publish called on ref, an interface of a named package.
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
