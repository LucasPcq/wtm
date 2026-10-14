package main

import (
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var (
	portPackages = []dir{"store", "infra/git"}
	portBuilders = []dir{"registry", "testutil"}
)

var portbuildAnalyzer = &analysis.Analyzer{
	Name: "portbuild",
	Doc:  "a real port (store.MetaDisk, git.Repo…) is built only in internal/registry and the test doubles",
	Run:  runPortbuild,
}

func runPortbuild(pass *analysis.Pass) (any, error) {
	pkg := pass.Pkg.Path()
	if slices.ContainsFunc(append(slices.Clone(portBuilders), portPackages...), func(d dir) bool { return d.holds(pkg) }) {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			if typ, ok := builtType(pass, node); ok {
				if port, isPort := realPort(typ); isPort {
					pass.Reportf(node.Pos(), "%s built in %s: a real port is built by internal/registry, on the dispatcher's directories, and handed to what uses it (docs/dev/lint.md, portbuild)", port, internalPath(pkg))
				}
			}
			return true
		})
	}
	return nil, nil
}

func builtType(pass *analysis.Pass, node ast.Node) (types.Type, bool) {
	switch n := node.(type) {
	case *ast.CompositeLit:
		return pass.TypesInfo.TypeOf(n), true
	case *ast.CallExpr:
		if builtin, ok := objectOf(pass.TypesInfo, ast.Unparen(n.Fun)).(*types.Builtin); ok && builtin.Name() == "new" && len(n.Args) == 1 {
			return pass.TypesInfo.TypeOf(n.Args[0]), true
		}
	case *ast.ValueSpec:
		if n.Type != nil && len(n.Values) == 0 {
			return pass.TypesInfo.TypeOf(n.Type), true
		}
	}
	return nil, false
}

func realPort(typ types.Type) (string, bool) {
	named, ok := typ.(*types.Named)
	if !ok || named.Obj().Pkg() == nil || types.IsInterface(named) {
		return "", false
	}
	owner := named.Obj().Pkg()
	if !slices.ContainsFunc(portPackages, func(d dir) bool { return d.holds(owner.Path()) }) {
		return "", false
	}
	for _, name := range owner.Scope().Names() {
		iface, ok := owner.Scope().Lookup(name).Type().Underlying().(*types.Interface)
		if ok && exportsMethod(iface) && types.Implements(types.NewPointer(named), iface) {
			return owner.Name() + "." + named.Obj().Name(), true
		}
	}
	return "", false
}
