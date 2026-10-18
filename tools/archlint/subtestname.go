package main

import (
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const nameField = "name"

var subtestnameAnalyzer = newSubtestnameAnalyzer(testFilesOnDisk)

func newSubtestnameAnalyzer(testsOf testFiles) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "subtestname",
		Doc:  "a subtest of next/ is named like an identifier: a string literal t.Run takes as its name, followed through a range over a literal table, or the value of a name field in a test's composite literal, has no space",
		Run:  func(pass *analysis.Pass) (any, error) { return runSubtestname(pass, testsOf) },
	}
}

// shortcut: a name is followed syntactically, within its test function and file, so a table declared in another file or a name built by fmt.Sprintf or a helper escapes; and the name: catch-all reads any field called name in a test, a branch's included. Resolve through go/types if either bites.
func runSubtestname(pass *analysis.Pass, testsOf testFiles) (any, error) {
	rel, ok := nextRel(pass)
	if !ok {
		return nil, nil
	}
	files, err := testsOf(pass)
	if err != nil {
		return nil, err
	}
	// A test kit runs subtests from its sources: every command's contract_test.go calls operationtest.Contract.
	if from, ok := nextPackageOf(rel); ok && slices.Contains(nextKits, from.layer) {
		files = append(files, sourcesOf(pass)...)
	}
	reported := map[token.Pos]bool{}
	check := func(lit *ast.BasicLit) {
		name, err := strconv.Unquote(lit.Value)
		if err != nil || !strings.Contains(name, " ") || reported[lit.Pos()] {
			return
		}
		reported[lit.Pos()] = true
		pass.Reportf(lit.Pos(), "subtest name %q has a space: name a subtest like an identifier, its description in a field of its own (Google \"Subtest names\", go-style)", name)
	}
	for _, file := range files {
		if isTestFile(pass, file) {
			ast.Inspect(file, func(node ast.Node) bool {
				if lit, ok := node.(*ast.CompositeLit); ok {
					for _, elt := range lit.Elts {
						if kv, ok := elt.(*ast.KeyValueExpr); ok && isIdent(kv.Key, nameField) {
							eachLiteral(kv.Value, check)
						}
					}
				}
				return true
			})
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			handles := testHandles(fn)
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok && isSubtest(call, handles) {
					for _, lit := range namesOf(call.Args[0], scope{fn: fn.Body, file: file, at: call.Pos()}) {
						check(lit)
					}
				}
				return true
			})
		}
	}
	return nil, nil
}

// testHandles returns the names fn and the function literals in it give a *testing.T or a *testing.B.
func testHandles(fn *ast.FuncDecl) map[string]bool {
	handles := map[string]bool{}
	ast.Inspect(fn, func(node ast.Node) bool {
		typ, ok := node.(*ast.FuncType)
		if !ok || typ.Params == nil {
			return true
		}
		for _, field := range typ.Params.List {
			star, ok := field.Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			sel, ok := star.X.(*ast.SelectorExpr)
			if !ok || !isIdent(sel.X, "testing") || sel.Sel.Name != "T" && sel.Sel.Name != "B" {
				continue
			}
			for _, name := range field.Names {
				handles[name.Name] = true
			}
		}
		return true
	})
	return handles
}

// isSubtest is true of t.Run(name, f) on a *testing.T or *testing.B the function holds, f a literal or any function value.
func isSubtest(call *ast.CallExpr, handles map[string]bool) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Run" || len(call.Args) != 2 {
		return false
	}
	recv, ok := sel.X.(*ast.Ident)
	return ok && handles[recv.Name]
}

// scope is where a subtest's name is looked for: the test function and its file, and the t.Run the name is given to.
type scope struct {
	fn   *ast.BlockStmt
	file *ast.File
	at   token.Pos
}

// namesOf returns the string literals a subtest's name is built from: the literal itself, the operands of a concatenation, and, for a range variable or a field of one, the matching keys, elements or field values of the literal table ranged over.
func namesOf(expr ast.Expr, in scope) []*ast.BasicLit {
	switch expr := ast.Unparen(expr).(type) {
	case *ast.BasicLit:
		return []*ast.BasicLit{expr}
	case *ast.BinaryExpr:
		if expr.Op == token.ADD {
			return append(namesOf(expr.X, in), namesOf(expr.Y, in)...)
		}
	case *ast.Ident:
		loop, key := rangeOf(expr.Name, in)
		if loop == nil {
			return literals(valueOf(expr.Name, in))
		}
		var lits []*ast.BasicLit
		for _, row := range rowsOf(loop.X, in) {
			if key {
				lits = append(lits, literals(row.key)...)
			} else {
				lits = append(lits, literals(row.value)...)
			}
		}
		return lits
	case *ast.SelectorExpr:
		v, ok := expr.X.(*ast.Ident)
		if !ok {
			return nil
		}
		loop, key := rangeOf(v.Name, in)
		if loop == nil || key {
			return nil
		}
		table := tableOf(loop.X, in)
		if table == nil {
			return nil
		}
		at := fieldIndex(rowType(table.Type, in), expr.Sel.Name)
		var lits []*ast.BasicLit
		for _, row := range rowsOf(loop.X, in) {
			lits = append(lits, literals(fieldOf(row.value, expr.Sel.Name, at))...)
		}
		return lits
	}
	return nil
}

// rangeOf returns the innermost range statement around the t.Run that declares name, and whether name is its key.
func rangeOf(name string, in scope) (*ast.RangeStmt, bool) {
	var found *ast.RangeStmt
	key := false
	ast.Inspect(in.fn, func(node ast.Node) bool {
		loop, ok := node.(*ast.RangeStmt)
		if !ok {
			return true
		}
		if loop.Body.Pos() > in.at || in.at >= loop.Body.End() {
			return false
		}
		switch {
		case isIdent(loop.Key, name):
			found, key = loop, true
		case isIdent(loop.Value, name):
			found, key = loop, false
		}
		return true
	})
	return found, key
}

type row struct{ key, value ast.Expr }

// rowsOf returns the rows of the table a range ranges over: a composite literal, or a variable the function or the file assigns one to.
func rowsOf(expr ast.Expr, in scope) []row {
	table := tableOf(expr, in)
	if table == nil {
		return nil
	}
	rows := make([]row, 0, len(table.Elts))
	for _, elt := range table.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			rows = append(rows, row{key: kv.Key, value: kv.Value})
			continue
		}
		rows = append(rows, row{value: elt})
	}
	return rows
}

func tableOf(expr ast.Expr, in scope) *ast.CompositeLit {
	if ident, ok := ast.Unparen(expr).(*ast.Ident); ok {
		expr = valueOf(ident.Name, in)
	}
	table, _ := ast.Unparen(expr).(*ast.CompositeLit)
	return table
}

// valueOf returns the value the test function, else its file, first assigns to name.
func valueOf(name string, in scope) ast.Expr {
	var value ast.Expr
	find := func(node ast.Node) bool {
		if value != nil {
			return false
		}
		switch node := node.(type) {
		case *ast.AssignStmt:
			for i, lhs := range node.Lhs {
				if isIdent(lhs, name) && i < len(node.Rhs) && len(node.Lhs) == len(node.Rhs) {
					value = node.Rhs[i]
				}
			}
		case *ast.ValueSpec:
			for i, n := range node.Names {
				if n.Name == name && i < len(node.Values) {
					value = node.Values[i]
				}
			}
		}
		return value == nil
	}
	ast.Inspect(in.fn, find)
	if value == nil {
		ast.Inspect(in.file, find)
	}
	return value
}

// rowType returns the struct type of a table's rows: a slice's or a map's elements, written inline or declared in the file.
func rowType(table ast.Expr, in scope) *ast.StructType {
	var elem ast.Expr
	switch table := table.(type) {
	case *ast.ArrayType:
		elem = table.Elt
	case *ast.MapType:
		elem = table.Value
	}
	if star, ok := elem.(*ast.StarExpr); ok {
		elem = star.X
	}
	if ident, ok := elem.(*ast.Ident); ok {
		ast.Inspect(in.file, func(node ast.Node) bool {
			if spec, ok := node.(*ast.TypeSpec); ok && spec.Name.Name == ident.Name {
				elem = spec.Type
			}
			return true
		})
	}
	typ, _ := elem.(*ast.StructType)
	return typ
}

// fieldIndex returns the position of field in typ, -1 when typ is unknown or has no such field.
func fieldIndex(typ *ast.StructType, field string) int {
	if typ == nil {
		return -1
	}
	at := 0
	for _, f := range typ.Fields.List {
		if len(f.Names) == 0 {
			at++
			continue
		}
		for _, name := range f.Names {
			if name.Name == field {
				return at
			}
			at++
		}
	}
	return -1
}

// fieldOf returns the value a row gives field: by key, else at its position in a row written without keys.
func fieldOf(expr ast.Expr, field string, at int) ast.Expr {
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		expr = unary.X
	}
	lit, ok := expr.(*ast.CompositeLit)
	if !ok || len(lit.Elts) == 0 {
		return nil
	}
	if _, keyed := lit.Elts[0].(*ast.KeyValueExpr); !keyed {
		if at < 0 || at >= len(lit.Elts) {
			return nil
		}
		return lit.Elts[at]
	}
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok && isIdent(kv.Key, field) {
			return kv.Value
		}
	}
	return nil
}

func literals(expr ast.Expr) []*ast.BasicLit {
	var lits []*ast.BasicLit
	eachLiteral(expr, func(lit *ast.BasicLit) { lits = append(lits, lit) })
	return lits
}

func eachLiteral(expr ast.Expr, do func(*ast.BasicLit)) {
	switch expr := ast.Unparen(expr).(type) {
	case *ast.BasicLit:
		if expr.Kind == token.STRING {
			do(expr)
		}
	case *ast.BinaryExpr:
		if expr.Op == token.ADD {
			eachLiteral(expr.X, do)
			eachLiteral(expr.Y, do)
		}
	}
}

func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}
