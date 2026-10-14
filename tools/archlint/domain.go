package main

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var domainAnalyzer = &analysis.Analyzer{
	Name: "domain",
	Doc:  "internal/domain declares types, errors and constants only",
	Run:  runDomain,
}

func runDomain(pass *analysis.Pass) (any, error) {
	if layerOfPackage(pass.Pkg.Path()) != "domain" {
		return nil, nil
	}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			pass.Reportf(fn.Pos(), "internal/domain declares %q: this layer holds types, errors and constants only — a function over the domain belongs in internal/rules", fn.Name.Name)
		}
	}
	return nil, nil
}
