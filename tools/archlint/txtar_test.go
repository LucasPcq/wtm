package main

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/txtar"
)

// runTxtar runs an analyzer over testdata/<archive>.txtar, each section named
// by its import path and laid out as a GOPATH tree, as x/tools does for its own.
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
