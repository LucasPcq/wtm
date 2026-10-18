package main

import (
	"go/ast"
	"go/token"
	"path/filepath"

	"golang.org/x/tools/go/analysis"
)

const portFile = "port.go"

var portfileAnalyzer = &analysis.Analyzer{
	Name: "portfile",
	Doc:  "a port.go of next/ declares only interface types and function types: the ports its package reaches the world through",
	Run:  runPortfile,
}

func runPortfile(pass *analysis.Pass) (any, error) {
	rel, ok := nextRel(pass)
	if !ok {
		return nil, nil
	}
	for _, file := range pass.Files {
		if filepath.Base(pass.Fset.File(file.Pos()).Name()) != portFile {
			continue
		}
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				pass.Reportf(decl.Name.Pos(), "next/%s/%s declares the function %s: a port.go declares only interface and function types, the code beside what uses it (next/README.md)", rel, portFile, decl.Name.Name)
			case *ast.GenDecl:
				reportPortDecl(pass, rel, decl)
			}
		}
	}
	return nil, nil
}

func reportPortDecl(pass *analysis.Pass, rel string, decl *ast.GenDecl) {
	switch decl.Tok {
	case token.IMPORT:
		return
	case token.TYPE:
		for _, spec := range decl.Specs {
			spec, ok := spec.(*ast.TypeSpec)
			if !ok || spec.Assign.IsValid() {
				pass.Reportf(decl.Pos(), "next/%s/%s declares a type alias: a port.go declares only interface and function types (next/README.md)", rel, portFile)
				continue
			}
			switch spec.Type.(type) {
			case *ast.InterfaceType, *ast.FuncType:
				continue
			}
			pass.Reportf(spec.Name.Pos(), "next/%s/%s declares %s, neither an interface nor a function type: a port.go declares only the ports, the types they take and return beside the code that owns them (next/README.md)", rel, portFile, spec.Name.Name)
		}
	default:
		pass.Reportf(decl.Pos(), "next/%s/%s declares a %s: a port.go declares only interface and function types (next/README.md)", rel, portFile, decl.Tok)
	}
}
