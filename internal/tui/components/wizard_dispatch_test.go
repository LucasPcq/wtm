package components

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
)

// notStepModels are the *Model types of this package that a Step never carries,
// so the dispatch switches have no business knowing them.
var notStepModels = []string{
	// The wizard is what dispatches; it is not dispatched to.
	"WizardModel",
	// Rendered by their own callers, never as a wizard step.
	"LoadingModel", "FilterModel", "BadgeModel", "BannerModel",
}

// stepModelTypes are the models declared in this package, read from the source
// rather than listed by hand: a table you must remember to extend guards nothing
// against the person who forgets. That is not hypothetical — EnvValueListModel
// was added to none of the switches, and the step rendered blank in a release
// build while every test passed.
func stepModelTypes(t *testing.T) []string {
	t.Helper()

	pkg := parsePackage(t)
	var models []string
	for _, file := range pkg {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || !strings.HasSuffix(ts.Name.Name, "Model") {
					continue
				}
				if _, isStruct := ts.Type.(*ast.StructType); !isStruct {
					continue
				}
				if !ts.Name.IsExported() || slices.Contains(notStepModels, ts.Name.Name) {
					continue
				}
				models = append(models, ts.Name.Name)
			}
		}
	}
	sort.Strings(models)
	return models
}

// parsePackage reads this package's own sources. The directory is walked here
// rather than through parser.ParseDir, which is deprecated: one package, one
// directory, no build tags to weigh.
func parsePackage(t *testing.T) []*ast.File {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	fset := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(fset, name, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		files = append(files, parsed)
	}
	if len(files) == 0 {
		t.Fatal("no sources parsed")
	}
	return files
}

// switchCases reads every type switch in wizard.go and returns the case types of
// each, so the sets can be compared against one another.
func switchCases(t *testing.T) [][]string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "wizard.go", nil, 0)
	if err != nil {
		t.Fatalf("parse wizard.go: %v", err)
	}

	var switches [][]string
	ast.Inspect(file, func(node ast.Node) bool {
		sw, ok := node.(*ast.TypeSwitchStmt)
		if !ok {
			return true
		}
		var cases []string
		for _, stmt := range sw.Body.List {
			clause, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			for _, expr := range clause.List {
				if ident, ok := expr.(*ast.Ident); ok && strings.HasSuffix(ident.Name, "Model") {
					cases = append(cases, ident.Name)
				}
			}
		}
		if len(cases) > 0 {
			sort.Strings(cases)
			switches = append(switches, cases)
		}
		return true
	})
	return switches
}

// Every step model must appear in every dispatch switch. A model missing from
// one renders blank, or swallows every key, or never resizes — and the step
// looks present while doing nothing at all.
func TestWizardDispatchesToEveryStepModel(t *testing.T) {
	models := stepModelTypes(t)
	if len(models) == 0 {
		t.Fatal("no step models found; the source scan is broken, not the wizard")
	}

	switches := switchCases(t)
	if len(switches) < 5 {
		t.Fatalf("found %d type switches in wizard.go, want the half-dozen that dispatch on Step.Model", len(switches))
	}

	for _, cases := range switches {
		for _, model := range models {
			if !slices.Contains(cases, model) {
				t.Errorf("%s is missing from a dispatch switch handling %v", model, cases)
			}
		}
	}
}
