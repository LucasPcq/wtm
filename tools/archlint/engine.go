package main

import (
	"go/ast"
	"slices"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

// The use cases of the engine reach nothing but their ports: a command never
// calls dispatch (a nested run would take its locks twice; a follow-up is a new
// call) nor a surface, and a surface reaches a command only through dispatch,
// so the locks, the fingerprint and the events always apply. Skip, Default and
// Validate are pure: a command has no root context to do I/O with.
var (
	useCases          = []dir{"command", "query"}
	useCaseForbidden  = []dir{"dispatch", "surface"}
	rootContexts      = []objectRef{{Path: "context", Name: "Background"}, {Path: "context", Name: "TODO"}}
	rootContextBanned = dir("command")
)

var engineAnalyzer = &analysis.Analyzer{
	Name: "engine",
	Doc:  "command/ and query/ never import dispatch or surface/, surface/ never imports them, and command/ never makes a root context",
	Run:  runEngine,
}

func runEngine(pass *analysis.Pass) (any, error) {
	own := pass.Pkg.Path()
	forbidden := forbiddenFor(own)
	for _, file := range pass.Files {
		for _, imp := range file.Imports {
			target, err := strconv.Unquote(imp.Path.Value)
			if err != nil || !slices.ContainsFunc(forbidden, func(d dir) bool { return d.holds(target) }) {
				continue
			}
			pass.Reportf(imp.Pos(), "%s must not import %q: a surface reaches a command through dispatch only, and a command never calls dispatch or a surface (docs/dev/dispatch.md)", internalPath(own), target)
		}
	}
	if !rootContextBanned.holds(own) {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			if !ok {
				return true
			}
			obj := pass.TypesInfo.Uses[ident]
			if slices.ContainsFunc(rootContexts, func(ref objectRef) bool { return refers(obj, ref) }) {
				pass.Reportf(ident.Pos(), "context.%s in command/: I/O takes the ctx it is handed, and Skip, Default and Validate do none", obj.Name())
			}
			return true
		})
	}
	return nil, nil
}

func forbiddenFor(pkgPath string) []dir {
	if slices.ContainsFunc(useCases, func(d dir) bool { return d.holds(pkgPath) }) {
		return useCaseForbidden
	}
	if dir("surface").holds(pkgPath) {
		return useCases
	}
	return nil
}
