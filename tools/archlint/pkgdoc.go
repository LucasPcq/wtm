package main

import (
	"go/ast"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var documented = []dir{"service", "infra", "store"}

var pkgdocAnalyzer = &analysis.Analyzer{
	Name: "pkgdoc",
	Doc:  "every package of service/, infra/ and store/ has a package doc stating its one responsibility",
	Run:  runPkgdoc,
}

func runPkgdoc(pass *analysis.Pass) (any, error) {
	if !slices.ContainsFunc(documented, func(d dir) bool { return d.holds(pass.Pkg.Path()) }) || len(pass.Files) == 0 {
		return nil, nil
	}
	if slices.ContainsFunc(pass.Files, func(file *ast.File) bool { return file.Doc != nil }) {
		return nil, nil
	}
	pass.Reportf(firstFile(pass).Package, "package %s has no package doc: say its one responsibility in a sentence (docs/dev/architecture.md, pkgdoc)", internalPath(pass.Pkg.Path()))
	return nil, nil
}
