package main

import (
	"go/ast"
	"go/parser"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const (
	nextModule = modulePath + "next"
	nextPrefix = nextModule + "/"
)

type nextLayer string

const (
	nextDomain      nextLayer = "domain"
	nextService     nextLayer = "service"
	nextServiceTest nextLayer = "service/test"
	nextCommand     nextLayer = "command"
	nextAdapter     nextLayer = "adapter"
	nextCLI         nextLayer = "command/cli"
	nextUI          nextLayer = "ui"
	nextAPI         nextLayer = "api"
	nextCmd         nextLayer = "cmd"
)

const (
	cliDir     = "cli"
	testSuffix = "test"
)

var sharedAdapters = []string{"statefile", "shell"}

type nextPackage struct {
	layer nextLayer
	unit  string
}

var nextWhy = map[nextLayer]string{
	nextDomain:      "the domain is pure: the stdlib and domain/* only",
	nextService:     "a service reaches the world through the ports it declares: domain/* and the stdlib, never another service, a command, an adapter or ui/*",
	nextCommand:     "a use case orchestrates services and reaches the world through the ports it declares: domain/*, service/* and the stdlib, never another command, its own cli, an adapter or ui/*",
	nextServiceTest: "a service's test kit checks what the service promises: service/<its name>, domain/*, the stdlib and third parties, never another service, a command, an adapter or ui/*",
	nextCLI:         "a command's cli drives its own use case: command/<its name>, domain/*, ui/*, the stdlib and third parties, never another command, a service or an adapter",
	nextAdapter:     "an adapter reaches domain/*, adapter/statefile, adapter/shell, the stdlib and third parties, never a service, a command or ui/*, nor another adapter",
	nextUI:          "ui/ is the presentation kit every command's cli shares and knows no command: domain/*, ui/*, the stdlib and third parties, never a command, a service or an adapter",
	nextAPI:         "the catalogue knows no command: domain/* and the stdlib only",
	nextCmd:         "the composition root wires what is under next/",
}

var nextAnalyzer = newNextAnalyzer(testFilesOnDisk)

func newNextAnalyzer(testsOf testFiles) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "next",
		Doc:  "a package of next/ imports only what its row of next/README.md allows, never internal/ (its tests neither), and a fake only from a test",
		Run:  func(pass *analysis.Pass) (any, error) { return runNext(pass, testsOf) },
	}
}

func runNext(pass *analysis.Pass, testsOf testFiles) (any, error) {
	rel, ok := nextRel(pass)
	if !ok {
		return nil, nil
	}
	from, ok := nextPackageOf(rel)
	if !ok {
		pass.Reportf(firstFile(pass).Package, "next/%s is in no row of next/'s import table: place it under domain/, service/, command/ (its cli/ beside it), adapter/, ui/, api/ or cmd/ (next/README.md)", rel)
		return nil, nil
	}
	sources := slices.DeleteFunc(slices.Clone(pass.Files), func(file *ast.File) bool { return isTestFile(pass, file) })
	for _, spec := range importsOf(sources) {
		target, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		if why, refused := nextRefuses(nextImport{From: from, Target: target}); refused {
			pass.Reportf(spec.Pos(), "next/%s must not import %q — %s (next/README.md)", rel, target, why)
		}
	}
	tests, err := testsOf(pass)
	if err != nil {
		return nil, err
	}
	for _, spec := range importsOf(tests) {
		if target, err := strconv.Unquote(spec.Path.Value); err == nil && strings.HasPrefix(target, internalPrefix) {
			pass.Reportf(spec.Pos(), "a test of next/%s must not import %q — next/ never imports internal/, its tests neither (next/README.md)", rel, target)
		}
	}
	return nil, nil
}

type testFiles func(pass *analysis.Pass) ([]*ast.File, error)

func isTestFile(pass *analysis.Pass, file *ast.File) bool {
	return strings.HasSuffix(pass.Fset.File(file.Pos()).Name(), "_test.go")
}

func testFilesOnDisk(pass *analysis.Pass) ([]*ast.File, error) {
	// archlint loads the packages without their tests: they are parsed into the
	// pass's file set, where a finding can point at them.
	dirPath := filepath.Dir(pass.Fset.File(pass.Files[0].Pos()).Name())
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(pass.Fset, filepath.Join(dirPath, entry.Name()), nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func importsOf(files []*ast.File) []*ast.ImportSpec {
	var specs []*ast.ImportSpec
	for _, file := range files {
		specs = append(specs, file.Imports...)
	}
	return specs
}

func nextRel(pass *analysis.Pass) (string, bool) {
	rel, ok := cutNext(pass.Pkg.Path())
	// analysistest also loads the main package go test generates for a test binary.
	generated := pass.Pkg.Name() == "main" && strings.HasSuffix(rel, ".test")
	return rel, ok && len(pass.Files) > 0 && !generated
}

func cutNext(pkgPath string) (string, bool) {
	if pkgPath == nextModule {
		return "", true
	}
	return strings.CutPrefix(pkgPath, nextPrefix)
}

func nextPackageOf(rel string) (nextPackage, bool) {
	parts := strings.Split(rel, "/")
	if len(parts) < 2 {
		return nextPackage{}, false
	}
	nested := len(parts) > 2
	unit := ""
	if nested {
		unit = parts[2]
	}
	switch {
	case parts[0] == "cmd":
		return nextPackage{layer: nextCmd}, true
	case parts[0] != "internal":
		return nextPackage{}, false
	case parts[1] == "api" && !nested:
		return nextPackage{layer: nextAPI}, true
	case parts[1] == "domain":
		return nextPackage{layer: nextDomain}, true
	case parts[1] == "service" && nested && strings.HasSuffix(unit, testSuffix):
		return nextPackage{layer: nextServiceTest, unit: unit}, true
	case parts[1] == "service" && nested:
		return nextPackage{layer: nextService, unit: unit}, true
	case parts[1] == "command" && len(parts) > 3 && parts[3] == cliDir:
		return nextPackage{layer: nextCLI, unit: unit}, true
	case parts[1] == "command" && nested:
		return nextPackage{layer: nextCommand, unit: unit}, true
	case parts[1] == "adapter" && nested:
		return nextPackage{layer: nextAdapter, unit: unit}, true
	case parts[1] == "ui" && nested:
		return nextPackage{layer: nextUI, unit: unit}, true
	}
	return nextPackage{}, false
}

type nextImport struct {
	From   nextPackage
	Target string
}

func nextRefuses(imp nextImport) (string, bool) {
	if strings.HasPrefix(imp.Target, internalPrefix) {
		return "next/ never imports internal/: copy what it needs and give it a home", true
	}
	rel, inNext := cutNext(imp.Target)
	to, inRow := nextPackageOf(rel)
	inNext = inNext && inRow
	fake := inNext && (to.layer == nextAdapter && strings.HasSuffix(to.unit, testSuffix) || to.layer == nextServiceTest)
	if fake && !(imp.From.layer == to.layer && imp.From.unit == to.unit) {
		return "a fake is imported from a _test.go only", true
	}
	if nextAdmits(imp, nextTarget{path: imp.Target, pkg: to, inNext: inNext}) {
		return "", false
	}
	return nextWhy[imp.From.layer], true
}

type nextTarget struct {
	path   string
	pkg    nextPackage
	inNext bool
}

func nextAdmits(imp nextImport, to nextTarget) bool {
	from := imp.From
	stdlib := !strings.Contains(strings.Split(to.path, "/")[0], ".")
	thirdParty := !stdlib && !strings.HasPrefix(to.path, modulePath)
	in := func(layers ...nextLayer) bool { return to.inNext && slices.Contains(layers, to.pkg.layer) }
	switch from.layer {
	case nextDomain:
		return stdlib || in(nextDomain)
	case nextService, nextAPI:
		return stdlib || in(nextDomain)
	case nextCommand:
		return stdlib || in(nextDomain, nextService) || in(nextCommand) && to.pkg.unit == from.unit
	case nextServiceTest:
		return stdlib || thirdParty || in(nextDomain) || in(nextService) && from.unit == to.pkg.unit+testSuffix
	case nextAdapter:
		return stdlib || thirdParty || in(nextDomain) || in(nextAdapter) && admitsAdapter(imp, to.pkg)
	case nextCLI:
		return stdlib || thirdParty || in(nextDomain, nextUI) || in(nextCommand) && to.pkg.unit == from.unit
	case nextUI:
		return stdlib || thirdParty || in(nextDomain, nextUI)
	case nextCmd:
		return stdlib || thirdParty || to.inNext
	}
	return false
}

func admitsAdapter(imp nextImport, to nextPackage) bool {
	from := imp.From
	return to.unit == from.unit || slices.Contains(sharedAdapters, to.unit) || from.unit == to.unit+testSuffix
}
