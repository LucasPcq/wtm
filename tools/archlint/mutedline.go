package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var (
	outputMessage = objectRef{Path: internalPrefix + "surface/cli/render", Name: "Message"}
	mutedStyle    = objectRef{Path: internalPrefix + "styles", Name: "Muted"}
)

var mutedlineAnalyzer = &analysis.Analyzer{
	Name: "mutedline",
	Doc:  "a bare line is never muted whole",
	Run:  runMutedLine,
}

func runMutedLine(pass *analysis.Pass) (any, error) {
	if !drawingLayers[layerOfPackage(pass.Pkg.Path())] {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 || !refers(objectOf(pass.TypesInfo, call.Fun), outputMessage) {
				return true
			}
			if rendersMutedWhole(pass.TypesInfo, call.Args[1]) {
				pass.Reportf(call.Pos(), "a bare line muted whole is the `=` register without its glyph: use render.Unchanged for a non-event, or subordinate detail with an indent — see docs/dev/output.md")
			}
			return true
		})
	}
	return nil, nil
}

// A muted fragment concatenated with content is a label, which is what Muted
// is for.
func rendersMutedWhole(info *types.Info, expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	render, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || render.Sel.Name != "Render" {
		return false
	}
	return refers(objectOf(info, render.X), mutedStyle)
}
