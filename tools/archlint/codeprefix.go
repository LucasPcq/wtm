package main

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

const (
	codeField = "Code"
	nameConst = "Name"
)

var codeprefixAnalyzer = &analysis.Analyzer{
	Name: "codeprefix",
	Doc:  "a code a command of next/ declares (the value given to a Code: field, or to a code constant or variable) starts with the command's Name and a dot",
	Run:  runCodeprefix,
}

// shortcut: only a value given to a Code: key or to a Code…/code… name is checked, so a code passed by position or through a helper (warn("x")) escapes, and a code… name that is no code (codeFence) is checked; type the codes (a domain.Code) if either bites.
func runCodeprefix(pass *analysis.Pass) (any, error) {
	rel, ok := nextRel(pass)
	if !ok {
		return nil, nil
	}
	from, ok := nextPackageOf(rel)
	if !ok || from.layer != nextCommand && from.layer != nextCLI {
		return nil, nil
	}
	name := commandName(pass, from.unit)
	prefix := name + "."
	check := func(expr ast.Expr) {
		expr = ast.Unparen(expr)
		// A constant named by reference is checked where it is declared.
		switch expr.(type) {
		case *ast.Ident, *ast.SelectorExpr:
			return
		}
		value := pass.TypesInfo.Types[expr].Value
		if value == nil || value.Kind() != constant.String || strings.HasPrefix(constant.StringVal(value), prefix) {
			return
		}
		pass.Reportf(expr.Pos(), "code %q is declared in command/%s: it must start with %q, or, shared by several commands, be a domain constant (next/README.md)", constant.StringVal(value), from.unit, prefix)
	}
	for _, file := range pass.Files {
		if isTestFile(pass, file) {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.KeyValueExpr:
				if key, ok := node.Key.(*ast.Ident); ok && key.Name == codeField {
					check(node.Value)
				}
			case *ast.ValueSpec:
				for i, name := range node.Names {
					if i < len(node.Values) && isCodeName(name.Name) {
						check(node.Values[i])
					}
				}
			}
			return true
		})
	}
	return nil, nil
}

// commandName returns the Name constant of command/<unit>, read from the package itself or from its import by the cli, else the directory's name.
func commandName(pass *analysis.Pass, unit string) string {
	path := nextPrefix + "internal/command/" + unit
	pkgs := append([]*types.Package{pass.Pkg}, pass.Pkg.Imports()...)
	for _, pkg := range pkgs {
		if pkg.Path() != path {
			continue
		}
		if name, ok := pkg.Scope().Lookup(nameConst).(*types.Const); ok && name.Val().Kind() == constant.String {
			return constant.StringVal(name.Val())
		}
	}
	return unit
}

// isCodeName is true of Code, codeRequired and CodePlanStale, not of codec.
func isCodeName(name string) bool {
	rest, ok := strings.CutPrefix(name, "Code")
	if !ok {
		rest, ok = strings.CutPrefix(name, "code")
	}
	return ok && (rest == "" || unicode.IsUpper(rune(rest[0])))
}
