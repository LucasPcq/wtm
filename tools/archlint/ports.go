package main

import (
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var portsAnalyzer = &analysis.Analyzer{
	Name: "ports",
	Doc:  "command/ and query/ declare no interface with an exported method: their ports are those of store/, infra/git and infra/proc",
	Run:  runPorts,
}

func runPorts(pass *analysis.Pass) (any, error) {
	if !slices.ContainsFunc(useCases, func(d dir) bool { return d.holds(pass.Pkg.Path()) }) {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			iface, ok := pass.TypesInfo.TypeOf(spec.Type).Underlying().(*types.Interface)
			if ok && exportsMethod(iface) {
				pass.Reportf(spec.Pos(), "interface %s declared in %s: a command's ports are those of store/, infra/git and infra/proc, whose writes writesink sees — take one of them, or a sealed interface for a closed set of values (docs/dev/lint.md, ports)", spec.Name.Name, internalPath(pass.Pkg.Path()))
			}
			return true
		})
	}
	return nil, nil
}

func exportsMethod(iface *types.Interface) bool {
	for method := range iface.Methods() {
		if method.Exported() {
			return true
		}
	}
	return false
}
