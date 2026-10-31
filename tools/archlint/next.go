package main

import (
	"cmp"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

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
	nextAdapterTest nextLayer = "adapter/test"
	nextUITest      nextLayer = "ui/test"
	nextCommand     nextLayer = "command"
	nextAdapter     nextLayer = "adapter"
	nextCLI         nextLayer = "command/cli"
	nextUI          nextLayer = "ui"
	nextAPI         nextLayer = "api"
	nextWeb         nextLayer = "web"
	nextDaemon      nextLayer = "daemon"
	nextCmd         nextLayer = "cmd"
)

const (
	cliDir     = "cli"
	testSuffix = "test"
	nextRoot   = "cmd/wtm-next"
)

var nextKits = []nextLayer{nextServiceTest, nextAdapterTest, nextUITest}

var sharedAdapters = []string{"statefile", "shell"}

type nextPackage struct {
	layer nextLayer
	unit  string
}

var nextWhy = map[nextLayer]string{
	nextDomain:      "the domain is pure: the stdlib and domain/* only",
	nextService:     "a service reaches the world through the ports it declares: domain/* and the stdlib, never another service, a command, an adapter or ui/*",
	nextCommand:     "a use case orchestrates services and reaches the world through the ports it declares: domain/*, service/* and the stdlib, never another command, its own cli, an adapter or ui/*",
	nextServiceTest: "a service's test kit checks what the service promises: service/<its name>, its own packages, domain/*, the stdlib and third parties, never another service, a command, an adapter or ui/*",
	nextCLI:         "a command's cli drives its own use case: command/<its name>, domain/*, ui/*, the stdlib and third parties, never another command, a service or an adapter",
	nextAdapter:     "an adapter reaches domain/*, adapter/statefile, adapter/shell, the stdlib and third parties, never a service, a command or ui/*, nor another adapter",
	nextAdapterTest: "an adapter's test kit reaches adapter/<its name>, its own packages, domain/*, adapter/statefile, adapter/shell, the stdlib and third parties, never a service, a command or ui/*, nor another adapter",
	nextUITest:      "a ui test kit reaches domain/*, ui/*, its own packages, the stdlib and third parties, never a command, a service or an adapter",
	nextUI:          "ui/ is the presentation kit every command's cli shares and knows no command: domain/*, ui/*, the stdlib and third parties, never a command, a service or an adapter",
	nextAPI:         "the catalogue knows no command: domain/* and the stdlib only",
	nextWeb:         "the web server speaks to the core through the catalogue alone: api, domain/* and the stdlib, never a command, a service, an adapter or ui/*",
	nextDaemon:      "the run daemon reaches the world through the ports it declares and is blind to git: domain/* and the stdlib, never a command, a service, an adapter, ui/*, the catalogue nor the web server",
	nextCmd:         "the composition root wires what is under next/, never a fake",
}

var nextAnalyzer = newNextAnalyzer(testFilesOnDisk)

func newNextAnalyzer(testsOf testFiles) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "next",
		Doc:  "a package of next/ sits in a row of next/README.md and imports only what that row allows, never internal/ (its tests neither), and a fake only from a test",
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
		pass.Reportf(firstFile(pass).Package, "next/%s is in no row of next/'s import table: place it under domain/, service/, command/ (its cli/ beside it), adapter/, ui/, api/, web/, daemon/ or cmd/wtm-next (next/README.md)", rel)
		return nil, nil
	}
	if sibling := strings.TrimSuffix(from.unit, testSuffix); slices.Contains(nextKits, from.layer) && !siblingExists(pass, rel, sibling) {
		pass.Reportf(firstFile(pass).Package, "next/%s: a name ending in %q is reserved for a test kit beside its package, and there is no %s beside it: rename it (next/README.md)", rel, testSuffix, sibling)
		return nil, nil
	}
	for _, spec := range importsOf(sourcesOf(pass)) {
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
	return strings.HasSuffix(baseName(pass, file), "_test.go")
}

func baseName(pass *analysis.Pass, file *ast.File) string {
	return filepath.Base(pass.Fset.File(file.Pos()).Name())
}

func sourcesOf(pass *analysis.Pass) []*ast.File {
	return slices.DeleteFunc(slices.Clone(pass.Files), func(file *ast.File) bool { return isTestFile(pass, file) })
}

type testsKey struct {
	fset *token.FileSet
	dir  string
}

// Several rules read a package's tests: each directory is parsed once per load, into the load's file set.
var parsedTests sync.Map

func testFilesOnDisk(pass *analysis.Pass) ([]*ast.File, error) {
	// archlint loads the packages without their tests: they are parsed into the
	// pass's file set, where a finding can point at them.
	dirPath := filepath.Dir(pass.Fset.File(pass.Files[0].Pos()).Name())
	parse, _ := parsedTests.LoadOrStore(testsKey{fset: pass.Fset, dir: dirPath}, sync.OnceValues(func() ([]*ast.File, error) {
		return parseTests(pass.Fset, dirPath)
	}))
	return parse.(func() ([]*ast.File, error))()
}

func parseTests(fset *token.FileSet, dirPath string) ([]*ast.File, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dirPath, entry.Name()), nil, parser.ParseComments)
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
	// analysistest also loads the main package go test generates for a test binary, and a
	// package's external tests as their own package: the driver reads those from disk.
	generated := pass.Pkg.Name() == "main" && strings.HasSuffix(rel, ".test")
	sources := slices.ContainsFunc(pass.Files, func(file *ast.File) bool { return !isTestFile(pass, file) })
	return rel, ok && sources && !generated
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
	case rel == nextRoot:
		return nextPackage{layer: nextCmd}, true
	case parts[0] != "internal":
		return nextPackage{}, false
	case parts[1] == "api" && !nested:
		return nextPackage{layer: nextAPI}, true
	case parts[1] == "web" && !nested:
		return nextPackage{layer: nextWeb}, true
	case parts[1] == "daemon" && !nested:
		return nextPackage{layer: nextDaemon}, true
	case parts[1] == "domain":
		return nextPackage{layer: nextDomain}, true
	case parts[1] == "service" && nested:
		return kitOr(nextService, nextServiceTest, unit), true
	case parts[1] == "command" && len(parts) > 3 && parts[3] == cliDir:
		return nextPackage{layer: nextCLI, unit: unit}, true
	case parts[1] == "command" && nested:
		return nextPackage{layer: nextCommand, unit: unit}, true
	case parts[1] == "adapter" && nested:
		return kitOr(nextAdapter, nextAdapterTest, unit), true
	case parts[1] == "ui" && nested:
		return kitOr(nextUI, nextUITest, unit), true
	}
	return nextPackage{}, false
}

func kitOr(layer, kit nextLayer, unit string) nextPackage {
	if strings.HasSuffix(unit, testSuffix) {
		return nextPackage{layer: kit, unit: unit}
	}
	return nextPackage{layer: layer, unit: unit}
}

func siblingExists(pass *analysis.Pass, rel, sibling string) bool {
	if sibling == "" {
		return false
	}
	// rel is internal/<layer>/<kit>[/sub…]: the layer's directory is that many levels above the package's.
	layerDir := filepath.Dir(pass.Fset.File(firstFile(pass).Pos()).Name())
	for range len(strings.Split(rel, "/")) - 2 {
		layerDir = filepath.Dir(layerDir)
	}
	info, err := os.Stat(filepath.Join(layerDir, sibling))
	return err == nil && info.IsDir()
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
	fake := inNext && slices.Contains(nextKits, to.layer)
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
	if slices.Contains(nextKits, from.layer) && in(from.layer) && to.pkg.unit == from.unit {
		return true
	}
	switch from.layer {
	case nextDomain:
		return stdlib || in(nextDomain)
	case nextService, nextAPI, nextDaemon:
		return stdlib || in(nextDomain)
	case nextWeb:
		return stdlib || in(nextDomain, nextAPI)
	case nextCommand:
		return stdlib || in(nextDomain, nextService) || in(nextCommand) && to.pkg.unit == from.unit
	case nextServiceTest:
		return stdlib || thirdParty || in(nextDomain) || in(nextService) && from.unit == to.pkg.unit+testSuffix
	case nextAdapter:
		return stdlib || thirdParty || in(nextDomain) || in(nextAdapter) && (to.pkg.unit == from.unit || slices.Contains(sharedAdapters, to.pkg.unit))
	case nextAdapterTest:
		return stdlib || thirdParty || in(nextDomain) || in(nextAdapter) && (slices.Contains(sharedAdapters, to.pkg.unit) || from.unit == to.pkg.unit+testSuffix)
	case nextCLI:
		return stdlib || thirdParty || in(nextDomain, nextUI) || in(nextCommand) && to.pkg.unit == from.unit
	case nextUI, nextUITest:
		return stdlib || thirdParty || in(nextDomain, nextUI)
	case nextCmd:
		return stdlib || thirdParty || to.inNext
	}
	return false
}

func testsAlone(roots []string) ([]finding, error) {
	// go/packages, loaded without tests, sees no package in a directory of tests alone: no analyzer reads its imports.
	var findings []finding
	for _, root := range roots {
		if clean := filepath.ToSlash(filepath.Clean(root)); clean != "next" && !strings.HasPrefix(clean, "next/") {
			continue
		}
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil || !entry.IsDir() {
				return err
			}
			if path != root && (entry.Name() == "testdata" || strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_")) {
				return filepath.SkipDir
			}
			test, err := onlyTests(path)
			if err != nil || test == "" {
				return err
			}
			findings = append(findings, finding{
				pos:  token.Position{Filename: filepath.Join(path, test), Line: 1, Column: 1},
				rule: nextAnalyzer.Name,
				msg:  filepath.ToSlash(filepath.Clean(path)) + " holds only tests, which no rule reads: a test sits beside the package it tests (next/README.md)",
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return findings, nil
}

// onlyTests returns the first test file of dir when no other Go file of it is compiled on a target system.
func onlyTests(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	first := ""
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			first = cmp.Or(first, name)
			continue
		}
		compiled, err := compiledOnATarget(dir, name)
		if err != nil || compiled {
			return "", err
		}
	}
	return first, nil
}

func compiledOnATarget(dir, name string) (bool, error) {
	for _, goos := range targetSystems {
		ctx := build.Default
		ctx.GOOS, ctx.CgoEnabled = goos, false
		match, err := ctx.MatchFile(dir, name)
		if err != nil || match {
			return match, err
		}
	}
	return false, nil
}
