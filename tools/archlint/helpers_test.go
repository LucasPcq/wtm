package main

import (
	"go/constant"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/txtar"
)

// Each section of the archive is named by its import path and laid out as a
// GOPATH tree, as x/tools does for its own.
func runTxtar(t *testing.T, a *analysis.Analyzer, archive string, patterns ...string) []*analysistest.Result {
	t.Helper()
	dir := t.TempDir()
	extract(t, filepath.Join(dir, "src"), archive)
	return analysistest.Run(t, dir, a, patterns...)
}

func extract(t *testing.T, root string, archive string) {
	t.Helper()
	parsed, err := txtar.ParseFile(filepath.Join("testdata", archive+".txtar"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range parsed.Files {
		path := filepath.Join(root, file.Name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, file.Data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func eventTypesOf(domain *types.Package) map[string]bool {
	declared := map[string]bool{}
	for _, name := range domain.Scope().Names() {
		c, ok := domain.Scope().Lookup(name).(*types.Const)
		if !ok || c.Val().Kind() != constant.String {
			continue
		}
		named, ok := c.Type().(*types.Named)
		if !ok || named.Obj().Name() != "EventType" {
			continue
		}
		declared[constant.StringVal(c.Val())] = true
	}
	return declared
}

func at(file string) token.Position { return token.Position{Filename: file, Line: 1, Column: 1} }
