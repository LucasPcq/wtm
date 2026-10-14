package main

import (
	"go/ast"
	"go/constant"
	"go/types"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

var (
	textErrors = []objectRef{{Path: "errors", Name: "New"}, {Path: "fmt", Name: "Errorf"}}
	kernelCode = objectRef{Path: internalPrefix + "kernel", Name: "Code"}
	formatVerb = regexp.MustCompile(`%[-+# 0-9.*\[\]]*[a-zA-Z%]`)
)

var enginetextAnalyzer = &analysis.Analyzer{
	Name: "enginetext",
	Doc:  "command/ and query/ build no error from English text and no kernel.Code from a value: the engine emits codes",
	Run:  runEnginetext,
}

func runEnginetext(pass *analysis.Pass) (any, error) {
	if !slices.ContainsFunc(useCases, func(d dir) bool { return d.holds(pass.Pkg.Path()) }) {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			obj := objectOf(pass.TypesInfo, call.Fun)
			if slices.ContainsFunc(textErrors, func(ref objectRef) bool { return refers(obj, ref) }) && hasWords(pass, call.Args[0]) {
				pass.Reportf(call.Pos(), "%s.%s with English text in %s: the engine emits a kernel.Code and its params, the catalogue holds the message (docs/dev/commands.md, enginetext)", obj.Pkg().Name(), obj.Name(), internalPath(pass.Pkg.Path()))
			}
			if typeName, ok := obj.(*types.TypeName); ok && refers(typeName, kernelCode) && pass.TypesInfo.Types[call.Args[0]].Value == nil {
				pass.Reportf(call.Pos(), "kernel.Code built from a value in %s: a code is a constant the catalogue holds a message for (docs/dev/commands.md, enginetext)", internalPath(pass.Pkg.Path()))
			}
			return true
		})
	}
	return nil, nil
}

func hasWords(pass *analysis.Pass, arg ast.Expr) bool {
	value := pass.TypesInfo.Types[arg].Value
	if value == nil || value.Kind() != constant.String {
		return false
	}
	return strings.IndexFunc(formatVerb.ReplaceAllString(constant.StringVal(value), ""), unicode.IsLetter) >= 0
}
