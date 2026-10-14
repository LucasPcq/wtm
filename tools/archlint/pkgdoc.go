package main

import (
	"go/ast"
	"slices"
	"sort"

	"golang.org/x/tools/go/analysis"
)

// documented are the layers whose every package states its one responsibility
// in its package doc: a sentence that needs an "and" is two packages.
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
	files := slices.Clone(pass.Files)
	sort.Slice(files, func(i, j int) bool {
		return pass.Fset.Position(files[i].Package).Filename < pass.Fset.Position(files[j].Package).Filename
	})
	pass.Reportf(files[0].Package, "package %s has no package doc: say its one responsibility in a sentence (docs/dev/architecture.md, pkgdoc)", internalPath(pass.Pkg.Path()))
	return nil, nil
}
