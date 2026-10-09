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

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestEveryCodeDeclaredHasItsMessage(t *testing.T) {
	declared := declaredCodes(t)
	if len(declared) == 0 {
		t.Fatal("found no kernel.Code constant: the scan is broken")
	}
	for code, where := range declared {
		if _, known := catalog[code]; !known {
			t.Errorf("%s declares code %q with no entry in the catalogue", where, code)
		}
	}
}

func TestTheCatalogueHoldsOnlyDeclaredCodes(t *testing.T) {
	declared := declaredCodes(t)
	for code := range catalog {
		if _, ok := declared[code]; !ok && code != requiredOneOf {
			t.Errorf("catalogue entry %q matches no declared code", code)
		}
	}
}

func TestAMessageFillsEveryParam(t *testing.T) {
	got := Message(kernel.CodeUndoFailed, map[string]string{kernel.ParamStep: "env", kernel.ParamLeft: "worktree,env"})
	if got != "could not undo env, left behind: worktree,env" {
		t.Errorf("got %q", got)
	}
}

func TestAFieldErrorReadsAsASentence(t *testing.T) {
	cases := []struct {
		problem kernel.FieldError
		want    string
	}{
		{kernel.FieldError{Path: "from", Code: kernel.CodeRequired}, "from is required"},
		{kernel.FieldError{Path: "a", Code: kernel.CodeRequired, With: []string{"b", "c"}}, "one of a, b, c is required"},
		{kernel.FieldError{Path: "push", Code: kernel.CodeExclusiveWith, With: []string{"no_push"}}, "push cannot be used with no_push"},
		{kernel.FieldError{Path: "mode", Code: kernel.CodeOneOf, Params: map[string]string{kernel.ParamValue: "x"}, Accepted: []string{"a", "b"}}, "mode must be one of a, b, not x"},
		{kernel.FieldError{Path: "from", Code: kernel.CodeRequired, Params: map[string]string{kernel.ParamBecause: string(kernel.CodeInternal)}}, "from is required: internal error"},
	}
	for _, c := range cases {
		if got := Field(c.problem); got != c.want {
			t.Errorf("Field(%+v) = %q, want %q", c.problem, got, c.want)
		}
	}
}

func TestAnUnknownCodeReadsAsItself(t *testing.T) {
	if got := Message("nowhere.declared", nil); got != "nowhere.declared" {
		t.Errorf("got %q", got)
	}
}

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
	if err != nil {
		t.Fatal(err)
	}
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
