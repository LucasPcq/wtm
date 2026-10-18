package main

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var sliceFiles = []string{"usecase.go", "port.go", "rules.go", "questions.go"}

var slicefilesAnalyzer = &analysis.Analyzer{
	Name: "slicefiles",
	Doc:  "the root of command/<name> in next/ holds only usecase.go, port.go, rules.go, questions.go, their tests and the cli/ directory",
	Run:  runSlicefiles,
}

func runSlicefiles(pass *analysis.Pass) (any, error) {
	rel, ok := nextRel(pass)
	if !ok {
		return nil, nil
	}
	from, ok := nextPackageOf(rel)
	if !ok || from.layer != nextCommand || rel != "internal/command/"+from.unit {
		return nil, nil
	}
	sources := slices.DeleteFunc(slices.Clone(pass.Files), func(file *ast.File) bool { return isTestFile(pass, file) })
	nameOf := func(file *ast.File) string { return pass.Fset.File(file.Pos()).Name() }
	at := map[string]token.Pos{}
	for _, file := range sources {
		at[filepath.Base(nameOf(file))] = file.Package
	}
	// A finding about an entry that is no source file points at the first source file by name, the same on every run.
	first := slices.MinFunc(sources, func(a, b *ast.File) int { return strings.Compare(nameOf(a), nameOf(b)) }).Package
	dirPath := filepath.Dir(pass.Fset.File(first).Name())
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		switch {
		case strings.HasPrefix(name, "."):
		case entry.IsDir() && name == cliDir:
		case !entry.IsDir() && (slices.Contains(sliceFiles, name) || strings.HasSuffix(name, "_test.go")):
		default:
			pos, ok := at[name]
			if !ok {
				pos = first
			}
			pass.Reportf(pos, "next/%s/%s: the root of a command holds only usecase.go, port.go, rules.go, questions.go, their tests and cli/; a decision goes in rules.go, a question in questions.go, a port in port.go, what two commands share in a service (next/README.md)", rel, name)
		}
	}
	return nil, nil
}
