package main

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

const codeField = "Code"

var codeprefixAnalyzer = &analysis.Analyzer{
	Name: "codeprefix",
	Doc:  "a code a command of next/ declares (a Code: literal, a code constant) starts with the command's name and a dot",
	Run:  runCodeprefix,
}

func runCodeprefix(pass *analysis.Pass) (any, error) {
	rel, ok := nextRel(pass)
	if !ok {
		return nil, nil
	}
	from, ok := nextPackageOf(rel)
	if !ok || from.layer != nextCommand && from.layer != nextCLI {
		return nil, nil
	}
	prefix := from.unit + "."
	check := func(lit *ast.BasicLit) {
		code, err := strconv.Unquote(lit.Value)
		if err != nil || strings.HasPrefix(code, prefix) {
			return
		}
		pass.Reportf(lit.Pos(), "code %q is declared in command/%s: it must start with %q, or, shared by several commands, be a domain constant (next/README.md)", code, from.unit, prefix)
	}
	for _, file := range pass.Files {
		if isTestFile(pass, file) {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.KeyValueExpr:
				key, ok := node.Key.(*ast.Ident)
				lit, isLit := node.Value.(*ast.BasicLit)
				if ok && isLit && key.Name == codeField && lit.Kind == token.STRING {
					check(lit)
				}
			case *ast.ValueSpec:
				for i, name := range node.Names {
					lit, isLit := valueAt(node, i).(*ast.BasicLit)
					if isLit && lit.Kind == token.STRING && isCodeName(name.Name) {
						check(lit)
					}
				}
			}
			return true
		})
	}
	return nil, nil
}

// isCodeName is true of Code, codeRequired and CodePlanStale, not of codec.
func isCodeName(name string) bool {
	rest, ok := strings.CutPrefix(name, "Code")
	if !ok {
		rest, ok = strings.CutPrefix(name, "code")
	}
	return ok && (rest == "" || unicode.IsUpper(rune(rest[0])))
}

func valueAt(spec *ast.ValueSpec, i int) ast.Expr {
	if i >= len(spec.Values) {
		return nil
	}
	return spec.Values[i]
}
