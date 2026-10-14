package main

import (
	"go/ast"
	"go/token"
	"path/filepath"

	"golang.org/x/tools/go/analysis"
)

var frozenRoots = []string{"rules", "domain"}

var rootfreezeAnalyzer = &analysis.Analyzer{
	Name: "rootfreeze",
	Doc:  "the roots of rules/ and domain/ take no new file nor exported symbol: new code goes into the sub-package of its subject",
	Run:  runRootfreeze,
}

func runRootfreeze(pass *analysis.Pass) (any, error) {
	root := internalPath(pass.Pkg.Path())
	if !frozen(root) {
		return nil, nil
	}
	for _, file := range pass.Files {
		pass.Reportf(file.Package, "%s at the root of %s/, which only shrinks: a new file goes into the sub-package of its subject (%s/<subject>; docs/dev/architecture.md, rootfreeze)", filepath.Base(pass.Fset.Position(file.Package).Filename), root, root)
		for _, name := range exportedNames(file) {
			pass.Reportf(name.Pos(), "%s.%s at the root of %s/, which only shrinks: put it in the sub-package of its subject (%s/<subject>; docs/dev/architecture.md, rootfreeze)", root, name.Name, root, root)
		}
	}
	return nil, nil
}

func frozen(pkg string) bool {
	for _, root := range frozenRoots {
		if pkg == root {
			return true
		}
	}
	return false
}

func exportedNames(file *ast.File) []*ast.Ident {
	var names []*ast.Ident
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.IsExported() {
				names = append(names, d.Name)
			}
		case *ast.GenDecl:
			names = append(names, exportedSpecs(d)...)
		}
	}
	return names
}

func exportedSpecs(decl *ast.GenDecl) []*ast.Ident {
	if decl.Tok == token.IMPORT {
		return nil
	}
	var names []*ast.Ident
	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if s.Name.IsExported() {
				names = append(names, s.Name)
			}
		case *ast.ValueSpec:
			for _, name := range s.Names {
				if name.IsExported() {
					names = append(names, name)
				}
			}
		}
	}
	return names
}
