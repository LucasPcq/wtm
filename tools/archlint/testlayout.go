package main

import (
	"go/ast"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"
)

// The files of a package's tests that hold what is not a test: the example and the fakes, the shared helpers, and what an external test reaches of the package.
var testSupportFiles = []string{"fixture_test.go", "helpers_test.go", "export_test.go"}

var testPrefixes = []string{"Test", "Benchmark", "Fuzz", "Example"}

var testlayoutAnalyzer = newTestlayoutAnalyzer(testFilesOnDisk)

func newTestlayoutAnalyzer(testsOf testFiles) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "testlayout",
		Doc:  "a top-level function of a _test.go of next/ is a Test, Benchmark, Fuzz or Example function, but in fixture_test.go, helpers_test.go and export_test.go",
		Run:  func(pass *analysis.Pass) (any, error) { return runTestlayout(pass, testsOf) },
	}
}

func runTestlayout(pass *analysis.Pass, testsOf testFiles) (any, error) {
	if _, ok := nextRel(pass); !ok {
		return nil, nil
	}
	tests, err := testsOf(pass)
	if err != nil {
		return nil, err
	}
	for _, file := range tests {
		if slices.Contains(testSupportFiles, filepath.Base(pass.Fset.File(file.Pos()).Name())) {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil && isTestFunc(fn.Name.Name) {
				continue
			}
			pass.Reportf(fn.Name.Pos(), "%s is no Test, Benchmark, Fuzz or Example function: a fake or the example goes in fixture_test.go, a shared helper in helpers_test.go, what an external test reaches in export_test.go (go-style)", fn.Name.Name)
		}
	}
	return nil, nil
}

// isTestFunc is true of a name go test runs: TestX, Test_x and Test itself, not Tests nor Testify.
func isTestFunc(name string) bool {
	for _, prefix := range testPrefixes {
		rest, ok := strings.CutPrefix(name, prefix)
		if !ok {
			continue
		}
		first, _ := utf8.DecodeRuneInString(rest)
		if rest == "" || !unicode.IsLower(first) {
			return true
		}
	}
	return false
}
