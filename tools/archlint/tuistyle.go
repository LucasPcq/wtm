package main

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var tuistyleAnalyzer = &analysis.Analyzer{
	Name: "tuistyle",
	Doc:  "internal/surface/cli/render uses no badge or dashboard style",
	Run:  runTUIStyle,
}

func runTUIStyle(pass *analysis.Pass) (any, error) {
	if layerOfPackage(pass.Pkg.Path()) != "surface/cli/render" {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			obj := pass.TypesInfo.Uses[sel.Sel]
			if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != internalPrefix+"styles" {
				return true
			}
			name := sel.Sel.Name
			if strings.HasPrefix(name, "Badge") || strings.HasPrefix(name, "Dashboard") {
				pass.Reportf(sel.Pos(), "internal/surface/cli/render must not use styles.%s: a badge is a TUI widget and a dashboard style belongs to that surface — a line of CLI output is text", name)
			}
			return true
		})
	}
	return nil, nil
}
