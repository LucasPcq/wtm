package domain

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// declared are the values of every constant this package declares of the type named typ, read from its source.
func declared(t *testing.T, typ string) []string {
	t.Helper()
	names, err := filepath.Glob("*.go")
	require.NoError(t, err)

	var values []string
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		require.NoError(t, err)
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if ident, typed := value.Type.(*ast.Ident); !ok || !typed || ident.Name != typ {
					continue
				}
				for _, literal := range value.Values {
					lit, ok := literal.(*ast.BasicLit)
					require.True(t, ok, "a %s is a string literal", typ)
					unquoted, err := strconv.Unquote(lit.Value)
					require.NoError(t, err)
					values = append(values, unquoted)
				}
			}
		}
	}

	return values
}
