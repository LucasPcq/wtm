package main

import (
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var mutationTargets = map[string][]string{
	"service/worktree":  {"Create", "Clean", "ForceClean", "Sync", "Relocate", "Extract", "Reparent", "Remove", "Move"},
	"service/env":       {"ApplyEnvSync", "ApplyEnvPorts", "WritePortKeys", "AddEnvTargets"},
	"service/compose":   {"PatchAll"},
	"service/runconfig": {"Save"},
}

var mutationAnalyzer = &analysis.Analyzer{
	Name: "mutation",
	Doc:  "a worktree-mutating command goes through internal/flow",
	Run:  runMutation,
}

func runMutation(pass *analysis.Pass) (any, error) {
	if layerOfPackage(pass.Pkg.Path()) != "commands" {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
			if !ok || !isMutationTarget(fn) {
				return true
			}
			pass.Reportf(sel.Pos(), "%s.%s is called from commands/: a worktree-mutating command goes through internal/flow/<cmd>, "+
				"or no second surface can ever run it — see CLAUDE.md, \"Every new worktree-mutating command goes through flow/\"", qualifierOf(sel, fn), fn.Name())
			return true
		})
	}
	return nil, nil
}

func isMutationTarget(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() != nil || fn.Pkg() == nil {
		return false
	}
	return slices.Contains(mutationTargets[internalPath(fn.Pkg().Path())], fn.Name())
}

// qualifierOf is the package as the caller spelled it, so the message points at
// the text a reader will search for.
func qualifierOf(sel *ast.SelectorExpr, fn *types.Func) string {
	if ident, ok := sel.X.(*ast.Ident); ok {
		return ident.Name
	}
	return fn.Pkg().Name()
}
