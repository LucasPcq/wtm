package main

import (
	"go/ast"
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
		Doc:  "a _test.go of next/ declares at top level only Test, Benchmark, Fuzz and Example functions (no other function, method, type or variable holding a function), but in fixture_test.go, helpers_test.go and export_test.go",
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
		if slices.Contains(testSupportFiles, baseName(pass, file)) {
			continue
		}
		for _, decl := range file.Decls {
			for _, name := range helpersIn(decl) {
				pass.Reportf(name.Pos(), "%s is no Test, Benchmark, Fuzz or Example function: a fake or the example goes in fixture_test.go, a shared helper in helpers_test.go, what an external test reaches in export_test.go (go-style)", name.Name)
			}
		}
	}
	return nil, nil
}

// helpersIn returns the names decl declares that are not tests: a function or a method that go test does not run, a type, a variable holding a function.
func helpersIn(decl ast.Decl) []*ast.Ident {
	var names []*ast.Ident
	switch decl := decl.(type) {
	case *ast.FuncDecl:
		if decl.Recv != nil || !isTestFunc(decl.Name.Name) {
			names = append(names, decl.Name)
		}
	case *ast.GenDecl:
		for _, spec := range decl.Specs {
			switch spec := spec.(type) {
			case *ast.TypeSpec:
				names = append(names, spec.Name)
			case *ast.ValueSpec:
				for i, name := range spec.Names {
					if i < len(spec.Values) {
						if _, ok := ast.Unparen(spec.Values[i]).(*ast.FuncLit); ok {
							names = append(names, name)
						}
					}
				}
			}
		}
	}
	return names
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
