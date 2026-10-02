package main

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

const lipglossPath = "github.com/charmbracelet/lipgloss"

var stylesAnalyzer = &analysis.Analyzer{
	Name: "styles",
	Doc:  "only internal/styles instantiates a lipgloss.Style",
	Run:  runStyles,
}

func runStyles(pass *analysis.Pass) (any, error) {
	if layerOfPackage(pass.Pkg.Path()) == "styles" {
		return nil, nil
	}
	newStyle := objectRef{Path: lipglossPath, Name: "NewStyle"}
	style := objectRef{Path: lipglossPath, Name: "Style"}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.CallExpr:
				if refers(objectOf(pass.TypesInfo, n.Fun), newStyle) {
					pass.Reportf(n.Pos(), "only internal/styles may instantiate a lipgloss.Style")
				}
			case *ast.CompositeLit:
				if refers(objectOf(pass.TypesInfo, n.Type), style) {
					pass.Reportf(n.Pos(), "only internal/styles may instantiate a lipgloss.Style")
				}
			}
			return true
		})
	}
	return nil, nil
}
