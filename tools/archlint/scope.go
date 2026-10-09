package main

import (
	"go/ast"
	"go/types"
	"strings"
)

const (
	modulePath     = "github.com/LucasPcq/wtm/"
	internalPrefix = modulePath + "internal/"
)

func internalPath(pkgPath string) string {
	rest, ok := strings.CutPrefix(pkgPath, internalPrefix)
	if !ok {
		return ""
	}
	return rest
}

// layerOfPackage is the longest row of the layers table holding the package:
// surface/cli/render sits inside surface/cli and is a layer of its own.
func layerOfPackage(pkgPath string) string {
	longest := ""
	for name := range layers {
		if len(name) > len(longest) && dir(name).holds(pkgPath) {
			longest = name
		}
	}
	if longest != "" {
		return longest
	}
	return strings.Split(internalPath(pkgPath), "/")[0]
}

type objectRef struct {
	Path string
	Name string
}

// objectOf resolves what an expression names through the type checker, so an
// aliased import names the same object as the plain one.
func objectOf(info *types.Info, expr ast.Expr) types.Object {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		return info.Uses[e.Sel]
	case *ast.Ident:
		return info.Uses[e]
	}
	return nil
}

// packageLevel tells shared.Interactive from a field of the same name in a
// shared params struct, which a command sets without reading any gate.
func packageLevel(obj types.Object) bool {
	return obj.Pkg().Scope().Lookup(obj.Name()) == obj
}

func refers(obj types.Object, ref objectRef) bool {
	return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == ref.Path && obj.Name() == ref.Name && packageLevel(obj)
}

// dir is a package directory under internal/, holding itself and its subpackages.
type dir string

func (d dir) holds(pkgPath string) bool {
	rel := internalPath(pkgPath)
	return rel == string(d) || strings.HasPrefix(rel, string(d)+"/")
}
