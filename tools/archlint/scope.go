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

func layerOfPackage(pkgPath string) string {
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

func refers(obj types.Object, ref objectRef) bool {
	return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == ref.Path && obj.Name() == ref.Name
}
