package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// metadataSinks are the two writes of a worktree's meta.json. A function that
// reaches one changes a worktree's identity, so it belongs in mutators — which
// is what makes that table complete by construction rather than by memory.
var metadataSinks = map[string]bool{"writeMetadata": true, "purgeState": true}

var metawriterAnalyzer = &analysis.Analyzer{
	Name: "metawriter",
	Doc:  "an exported function of service/worktree that writes metadata is in the mutators table (methods are out of scope)",
	Run:  runMetawriter,
}

func runMetawriter(pass *analysis.Pass) (any, error) {
	if internalPath(pass.Pkg.Path()) != "service/worktree" {
		return nil, nil
	}
	calls, decls := callGraph(pass)
	writes := reachingSinks(calls)
	for fn, decl := range decls {
		if !fn.Exported() || !writes[fn] || isMutator(fn) {
			continue
		}
		if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
			continue
		}
		pass.Reportf(decl.Pos(), "worktree.%s writes worktree metadata but is not in the mutators table (tools/archlint/chokepoint.go): "+
			"a change to a worktree's identity goes through a flow that publishes it", fn.Name())
	}
	return nil, nil
}

// callGraph maps each function declared in the package to the functions of the
// same package it calls, closures included.
func callGraph(pass *analysis.Pass) (map[*types.Func][]*types.Func, map[*types.Func]*ast.FuncDecl) {
	calls := map[*types.Func][]*types.Func{}
	decls := map[*types.Func]*ast.FuncDecl{}
	for _, file := range pass.Files {
		for _, d := range file.Decls {
			decl, ok := d.(*ast.FuncDecl)
			if !ok || decl.Body == nil {
				continue
			}
			fn, ok := pass.TypesInfo.Defs[decl.Name].(*types.Func)
			if !ok {
				continue
			}
			decls[fn] = decl
			ast.Inspect(decl.Body, func(node ast.Node) bool {
				ident, ok := node.(*ast.Ident)
				if !ok {
					return true
				}
				callee, ok := pass.TypesInfo.Uses[ident].(*types.Func)
				if ok && callee.Pkg() == pass.Pkg {
					calls[fn] = append(calls[fn], callee)
				}
				return true
			})
		}
	}
	return calls, decls
}

func reachingSinks(calls map[*types.Func][]*types.Func) map[*types.Func]bool {
	writes := map[*types.Func]bool{}
	for changed := true; changed; {
		changed = false
		for fn, callees := range calls {
			if writes[fn] {
				continue
			}
			for _, callee := range callees {
				if metadataSinks[callee.Name()] || writes[callee] {
					writes[fn] = true
					changed = true
					break
				}
			}
		}
	}
	return writes
}
