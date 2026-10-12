package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// metaWrites are the writes of a worktree's meta.json, methods of store.Meta
// and of its implementations. A function of service/worktree that reaches one
// changes a worktree's identity, so it belongs in mutators — which is what
// makes that table complete by construction rather than by memory.
var metaWrites = map[string]bool{"Update": true, "Purge": true}

var metaWriters = map[string]bool{"Meta": true, "MetaDisk": true}

var metawriterAnalyzer = &analysis.Analyzer{
	Name: "metawriter",
	Doc:  "an exported function of service/worktree that writes metadata (store.Meta's Update or Purge) is in the mutators table (methods are out of scope)",
	Run:  runMetawriter,
}

func runMetawriter(pass *analysis.Pass) (any, error) {
	if internalPath(pass.Pkg.Path()) != "service/worktree" {
		return nil, nil
	}
	graph := callGraph(pass)
	writes := reachingSinks(graph)
	for fn, decl := range graph.decls {
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

type packageCalls struct {
	// calls maps each function declared in the package to the functions of
	// the same package it calls, closures included.
	calls map[*types.Func][]*types.Func
	// sinks are the functions that call a write of store.Meta themselves.
	sinks map[*types.Func]bool
	decls map[*types.Func]*ast.FuncDecl
}

func callGraph(pass *analysis.Pass) packageCalls {
	graph := packageCalls{calls: map[*types.Func][]*types.Func{}, sinks: map[*types.Func]bool{}, decls: map[*types.Func]*ast.FuncDecl{}}
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
			graph.decls[fn] = decl
			ast.Inspect(decl.Body, func(node ast.Node) bool {
				ident, ok := node.(*ast.Ident)
				if !ok {
					return true
				}
				callee, ok := pass.TypesInfo.Uses[ident].(*types.Func)
				switch {
				case !ok:
				case callee.Pkg() == pass.Pkg:
					graph.calls[fn] = append(graph.calls[fn], callee)
				case isMetaWrite(callee):
					graph.sinks[fn] = true
				}
				return true
			})
		}
	}
	return graph
}

func isMetaWrite(fn *types.Func) bool {
	if fn.Pkg() == nil || internalPath(fn.Pkg().Path()) != "store" || !metaWrites[fn.Name()] {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	recv := sig.Recv().Type()
	if pointer, ok := recv.(*types.Pointer); ok {
		recv = pointer.Elem()
	}
	named, ok := recv.(*types.Named)
	return ok && metaWriters[named.Obj().Name()]
}

func reachingSinks(graph packageCalls) map[*types.Func]bool {
	writes := map[*types.Func]bool{}
	for fn := range graph.sinks {
		writes[fn] = true
	}
	for changed := true; changed; {
		changed = false
		for fn, callees := range graph.calls {
			if writes[fn] {
				continue
			}
			for _, callee := range callees {
				if writes[callee] {
					writes[fn] = true
					changed = true
					break
				}
			}
		}
	}
	return writes
}
