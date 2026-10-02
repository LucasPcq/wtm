package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// mutators are the service calls that change a worktree. Each is called from
// internal/flow only, whatever the caller's layer: a command, the dashboard or
// another service reaching one directly is a mutation no flow sees, so no
// second surface can run it and no event can report it. A call inside the
// mutator's own package is its implementation, not an escape.
//
// event is the domain event a flow publishes after the call; LUC-233 fills it.
type mutator struct {
	pkg   string
	name  string
	event string
}

var mutators = []mutator{
	{pkg: "service/worktree", name: "Create", event: ""},
	{pkg: "service/worktree", name: "Clean", event: ""},
	{pkg: "service/worktree", name: "ForceClean", event: ""},
	{pkg: "service/worktree", name: "FinishRemoval", event: ""},
	{pkg: "service/worktree", name: "Move", event: ""},
	{pkg: "service/worktree", name: "Adopt", event: ""},
	{pkg: "service/worktree", name: "SetBasePath", event: ""},
	{pkg: "service/worktree", name: "ReparentBatch", event: ""},
	{pkg: "service/worktree", name: "ApplyReparents", event: ""},
	{pkg: "service/worktree", name: "SetIsolation", event: ""},
	{pkg: "service/worktree", name: "EnsureOrdinal", event: ""},
	{pkg: "service/worktree", name: "RecordNamespaces", event: ""},
	{pkg: "service/worktree", name: "Sync", event: ""},
	{pkg: "service/worktree", name: "Extract", event: ""},
	{pkg: "service/env", name: "ApplyEnvSync", event: ""},
	{pkg: "service/env", name: "ApplyEnvPorts", event: ""},
	{pkg: "service/env", name: "WritePortKeys", event: ""},
	{pkg: "service/env", name: "AddEnvTargets", event: ""},
	{pkg: "service/compose", name: "PatchAll", event: ""},
	{pkg: "service/runconfig", name: "Save", event: ""},
}

var chokepointAnalyzer = &analysis.Analyzer{
	Name: "chokepoint",
	Doc:  "a service mutator is called from internal/flow only",
	Run:  runChokepoint,
}

func runChokepoint(pass *analysis.Pass) (any, error) {
	if dir("flow").holds(pass.Pkg.Path()) {
		return nil, nil
	}
	caller := layerOfPackage(pass.Pkg.Path())
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
			if !ok || fn.Pkg() == pass.Pkg || !isMutator(fn) {
				return true
			}
			pass.Reportf(sel.Pos(), "%s.%s is called from %s/: a worktree-mutating command goes through internal/flow/<cmd>, "+
				"or no second surface can ever run it — see CLAUDE.md, \"Every new worktree-mutating command goes through flow/\"", qualifierOf(sel, fn), fn.Name(), caller)
			return true
		})
	}
	return nil, nil
}

func isMutator(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() != nil || fn.Pkg() == nil {
		return false
	}
	pkg := internalPath(fn.Pkg().Path())
	for _, m := range mutators {
		if m.pkg == pkg && m.name == fn.Name() {
			return true
		}
	}
	return false
}

// qualifierOf is the package as the caller spelled it, so the message points at
// the text a reader will search for.
func qualifierOf(sel *ast.SelectorExpr, fn *types.Func) string {
	if ident, ok := sel.X.(*ast.Ident); ok {
		return ident.Name
	}
	return fn.Pkg().Name()
}
