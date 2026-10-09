package text

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
)

// declaredCodes scans every package under internal/ for constants typed
// Code (in kernel) or kernel.Code (elsewhere).
func declaredCodes(t *testing.T) map[kernel.Code]string {
	t.Helper()
	root := filepath.Join("..", "..")
	declared := map[kernel.Code]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for code := range codesIn(file) {
			declared[code] = path
		}
		return nil
	})
	require.NoError(t, err)
	return declared
}

func codesIn(file *ast.File) map[kernel.Code]bool {
	codes := map[kernel.Code]bool{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || !isCodeType(value.Type, file.Name.Name) {
				continue
			}
			for _, expr := range value.Values {
				literal, ok := expr.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					continue
				}
				unquoted, err := strconv.Unquote(literal.Value)
				if err == nil {
					codes[kernel.Code(unquoted)] = true
				}
			}
		}
	}
	return codes
}

func isCodeType(expr ast.Expr, pkg string) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name == "Code" && pkg == "kernel"
	}
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	return ok && qualifier.Name == "kernel" && selector.Sel.Name == "Code"
}
