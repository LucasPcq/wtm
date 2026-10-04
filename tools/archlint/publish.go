package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var (
	processPublish = objectRef{Path: internalPrefix + "service/process", Name: "Publish"}
	// busSeams are the flow types whose Publish only a flow may call: an event
	// published from anywhere else reports a change no flow made, which no other
	// surface could replay.
	busSeams = map[string]bool{"Publisher": true, "Context": true}
)

var publishAnalyzer = &analysis.Analyzer{
	Name: "publish",
	Doc:  "process.Publish is called from service/events only, and the flow seam's Publish from internal/flow only",
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
			if refers(pass.TypesInfo.Uses[sel.Sel], processPublish) && !dir("service/events").holds(pkg) && !dir("service/process").holds(pkg) {
				pass.Reportf(sel.Pos(), "process.Publish is called from %s: only service/events publishes, so every event goes out stamped and versioned", internalPath(pkg))
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
