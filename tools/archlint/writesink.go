package main

import (
	"go/ast"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var engineServices = []dir{"service/gitwt", "service/identity", "service/runenv", "service/hooks"}

var mutatorCallers = []dir{"command", "service"}

var readOnly = []dir{"query"}

var readerFields = map[objectRef][]string{
	{Path: internalPrefix + "kernel", Name: "Command"}:   {"Observe", "Forget", "Locks", "Plan"},
	{Path: internalPrefix + "kernel", Name: "FieldDef"}:  {"Skip", "Default", "Validate", "Choices"},
	{Path: internalPrefix + "kernel", Name: "Query"}:     {"Run", "Stream"},
	{Path: internalPrefix + "kernel", Name: "Rule"}:      {"Check"},
	{Path: internalPrefix + "kernel", Name: "Condition"}: {"Holds"},
	{Path: internalPrefix + "dispatch", Name: "Events"}:  {"Committed", "PhaseDone", "Finished"},
}

var applyField = objectRef{Path: internalPrefix + "kernel", Name: "Command"}

const writingField = "Apply"

var writesinkAnalyzer = &analysis.Analyzer{
	Name:      "writesink",
	Doc:       "a function that reaches a write sink is a mutator: never called from query/ or from a reader of the engine's contract, and a mutator of the engine's services only from command/",
	Run:       runWritesink,
	FactTypes: []analysis.Fact{new(mutatorFact)},
}

func runWritesink(pass *analysis.Pass) (any, error) {
	pkg := pass.Pkg.Path()
	if internalPath(pkg) == "" {
		return nil, nil
	}
	if dir("infra").holds(pkg) {
		exportPortFacts(pass)
		return nil, nil
	}
	writes := mutatorsOf(pass)
	for fn, via := range writes {
		pass.ExportObjectFact(fn, &mutatorFact{Via: via})
	}
	exportInterfaceFacts(pass, writes)
	check := writeCheck{pass: pass, writes: writes}
	if slices.ContainsFunc(readOnly, func(d dir) bool { return d.holds(pkg) }) {
		check.readOnlyPackage()
		return nil, nil
	}
	check.readerFields()
	check.engineMutatorCalls()
	return nil, nil
}

func written(path string) string {
	name, via, ok := strings.Cut(path, " → ")
	if !ok {
		return name + " writes"
	}
	return name + " writes (" + via + ")"
}

type writeCheck struct {
	pass   *analysis.Pass
	writes map[*types.Func]string
}

func (c writeCheck) readOnlyPackage() {
	for _, file := range c.pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			c.refuse(node, "%s, in %s: a query only reads (docs/dev/lint.md, writesink)", internalPath(c.pass.Pkg.Path()))
			return true
		})
	}
}

func (c writeCheck) readerFields() {
	for _, file := range c.pass.Files {
		inApply := map[ast.Expr]bool{}
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case ast.Expr:
				if inApply[n] {
					return false
				}
				if lit, ok := n.(*ast.CompositeLit); ok {
					c.contractLiteral(contractLiteralParams{Lit: lit, InApply: inApply})
				}
			case *ast.AssignStmt:
				c.contractAssignment(n)
			}
			return true
		})
	}
}

type contractLiteralParams struct {
	Lit     *ast.CompositeLit
	InApply map[ast.Expr]bool
}

func (c writeCheck) contractLiteral(params contractLiteralParams) {
	holder, readers := contractOf(c.pass.TypesInfo.TypeOf(params.Lit))
	for _, elt := range params.Lit.Elts {
		field, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := field.Key.(*ast.Ident)
		if !ok {
			continue
		}
		if holder == applyField && key.Name == writingField {
			params.InApply[field.Value] = true
			continue
		}
		if slices.Contains(readers, key.Name) {
			c.readerBody(key.Name, field.Value)
		}
	}
}

func (c writeCheck) contractAssignment(assign *ast.AssignStmt) {
	if len(assign.Lhs) != len(assign.Rhs) {
		return
	}
	for index, lhs := range assign.Lhs {
		sel, ok := lhs.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		_, readers := contractOf(c.pass.TypesInfo.TypeOf(sel.X))
		if slices.Contains(readers, sel.Sel.Name) {
			c.readerBody(sel.Sel.Name, assign.Rhs[index])
		}
	}
}

func contractOf(typ types.Type) (objectRef, []string) {
	if pointer, ok := typ.(*types.Pointer); ok {
		typ = pointer.Elem()
	}
	named, ok := typ.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return objectRef{}, nil
	}
	ref := objectRef{Path: named.Obj().Pkg().Path(), Name: named.Obj().Name()}
	return ref, readerFields[ref]
}

func (c writeCheck) readerBody(field string, value ast.Expr) {
	switch v := ast.Unparen(value).(type) {
	case *ast.FuncLit:
		ast.Inspect(v.Body, func(node ast.Node) bool {
			c.refuse(node, "%s, in "+field+": only Apply writes, the rest of a command reads (docs/dev/lint.md, writesink)")
			return true
		})
	case *ast.CallExpr:
		if name := funcName(v.Fun); name != nil {
			c.refuse(name, "%s, building "+field+": only Apply writes, the rest of a command reads (docs/dev/lint.md, writesink)")
		}
	default:
		if name := funcName(v); name != nil {
			c.refuse(name, "%s, as "+field+": only Apply writes, the rest of a command reads (docs/dev/lint.md, writesink)")
		}
	}
}

func funcName(expr ast.Expr) *ast.Ident {
	switch e := ast.Unparen(expr).(type) {
	case *ast.Ident:
		return e
	case *ast.SelectorExpr:
		return e.Sel
	case *ast.IndexExpr:
		return funcName(e.X)
	case *ast.IndexListExpr:
		return funcName(e.X)
	}
	return nil
}

func (c writeCheck) engineMutatorCalls() {
	if slices.ContainsFunc(mutatorCallers, func(d dir) bool { return d.holds(c.pass.Pkg.Path()) }) {
		return
	}
	for _, file := range c.pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			callee, ok := usedFunc(c.pass, node)
			if !ok || callee.Pkg() == nil || !slices.ContainsFunc(engineServices, func(d dir) bool { return d.holds(callee.Pkg().Path()) }) {
				return true
			}
			if path, ok := writeOf(c.pass, c.writes, callee); ok {
				c.pass.Reportf(node.Pos(), "%s, called from %s: a mutator of the engine's services is called from command/ or another service only (docs/dev/services.md, writesink)", written(path), internalPath(c.pass.Pkg.Path()))
			}
			return true
		})
	}
}

func (c writeCheck) refuse(node ast.Node, format string, args ...any) {
	callee, ok := usedFunc(c.pass, node)
	if !ok {
		return
	}
	if path, ok := writeOf(c.pass, c.writes, callee); ok {
		c.pass.Reportf(node.Pos(), format, append([]any{written(path)}, args...)...)
	}
}
