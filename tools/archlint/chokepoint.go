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
// events are the domain events a flow publishes after the call (rule emits);
// none for a change that is not part of a worktree's identity.
type mutator struct {
	pkg    string
	name   string
	events []string
}

var mutators = []mutator{
	{pkg: "service/worktree", name: "Create", events: []string{"worktree.created", "worktree.provisioned"}},
	{pkg: "service/worktree", name: "Clean", events: []string{"worktree.removed"}},
	{pkg: "service/worktree", name: "ForceClean", events: []string{"worktree.removed"}},
	{pkg: "service/worktree", name: "FinishRemoval", events: []string{"worktree.removed"}},
	{pkg: "service/worktree", name: "Move", events: []string{"worktree.relocated"}},
	{pkg: "service/worktree", name: "Adopt", events: []string{"worktree.updated"}},
	{pkg: "service/worktree", name: "SetBasePath"},
	{pkg: "service/worktree", name: "ReparentBatch", events: []string{"worktree.reparented"}},
	{pkg: "service/worktree", name: "ApplyReparents", events: []string{"worktree.reparented"}},
	{pkg: "service/worktree", name: "SetIsolation", events: []string{"worktree.updated"}},
	{pkg: "service/worktree", name: "EnsureOrdinal", events: []string{"worktree.updated"}},
	{pkg: "service/worktree", name: "RecordNamespaces"},
	{pkg: "service/worktree", name: "Sync"},
	{pkg: "service/worktree", name: "Extract"},
	{pkg: "service/env", name: "ApplyEnvSync"},
	{pkg: "service/env", name: "ApplyEnvPorts"},
	{pkg: "service/env", name: "WritePortKeys"},
	{pkg: "service/env", name: "AddEnvTargets"},
	{pkg: "service/compose", name: "PatchAll"},
	{pkg: "service/runconfig", name: "Save"},
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
