package main

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

var typeassertAnalyzer = &analysis.Analyzer{
	Name: "typeassert",
	Doc:  "a type assertion uses the comma-ok form",
	Run:  runTypeAssert,
}

func runTypeAssert(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, pos := range unguardedAssertions(file) {
			pass.Reportf(pos, "type assertion without the comma-ok form: a wrong type must be an error, not a panic")
		}
	}
	return nil, nil
}

// unguardedAssertions is every `x.(T)` a wrong type would panic on. The two
// checked forms are subtracted rather than special-cased on the way down: a
// type switch owns its assertion, and a comma-ok one is only recognisable from
// the assignment above it.
func unguardedAssertions(file *ast.File) []token.Pos {
	guarded := map[token.Pos]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		var values []ast.Expr
		var targets int
		switch n := node.(type) {
		case *ast.AssignStmt:
			values, targets = n.Rhs, len(n.Lhs)
		case *ast.ValueSpec:
			values, targets = n.Values, len(n.Names)
		case *ast.TypeSwitchStmt:
			ast.Inspect(n.Assign, func(inner ast.Node) bool {
				if assert, ok := inner.(*ast.TypeAssertExpr); ok {
					guarded[assert.Pos()] = true
				}
				return true
			})
			return true
		default:
			return true
		}
		if targets != 2 || len(values) != 1 {
			return true
		}
		if assert, ok := values[0].(*ast.TypeAssertExpr); ok {
			guarded[assert.Pos()] = true
		}
		return true
	})

	var unguarded []token.Pos
	ast.Inspect(file, func(node ast.Node) bool {
		assert, ok := node.(*ast.TypeAssertExpr)
		// A nil Type is the `x.(type)` of a type switch, which has no wrong-type
		// case to guard.
		if ok && assert.Type != nil && !guarded[assert.Pos()] {
			unguarded = append(unguarded, assert.Pos())
		}
		return true
	})
	return unguarded
}
