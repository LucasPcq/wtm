package main

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"regexp"
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
	nextDomain  nextLayer = "domain"
	nextSlice   nextLayer = "slice"
	nextApp     nextLayer = "app"
	nextAdapter nextLayer = "adapter"
	nextSurface nextLayer = "surface"
	nextCmd     nextLayer = "cmd"
)

const sharedSlice = "provision"

var sharedAdapters = []string{"statefile", "shell"}

type nextPackage struct {
	layer nextLayer
	unit  string
}

var nextWhy = map[nextLayer]string{
	nextDomain:  "the domain is pure: the stdlib and wtm/* only",
	nextSlice:   "a use case reaches the world through the interfaces it declares: wtm/*, app/provision and the stdlib, never another slice, an adapter, a surface or app",
	nextApp:     "the API every surface calls: app/*, wtm/* and the stdlib, never an adapter",
	nextAdapter: "an adapter reaches wtm/*, adapter/statefile, adapter/shell, the stdlib and third parties, never app/*, a surface or another adapter",
	nextSurface: "a surface calls the app: app, app/<slice>, wtm/*, surface/* and third parties, never an adapter",
	nextCmd:     "the composition root wires what is under next/",
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
		pass.Reportf(firstFile(pass).Package, "next/%s is in no row of next/'s import table: place it under wtm/, app/, adapter/, surface/ or cmd/ (next/README.md)", rel)
		return nil, nil
	}
	fakes, err := fakesOf(nextRoot(pass, rel))
	if err != nil {
		return nil, err
	}
	sources := slices.DeleteFunc(slices.Clone(pass.Files), func(file *ast.File) bool { return isTestFile(pass, file) })
	for _, spec := range importsOf(sources) {
		target, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		if why, refused := nextRefuses(nextImport{From: from, Target: target, Fakes: fakes}); refused {
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
	case parts[1] == "wtm":
		return nextPackage{layer: nextDomain}, true
	case parts[1] == "app" && !nested:
		return nextPackage{layer: nextApp}, true
	case parts[1] == "app":
		return nextPackage{layer: nextSlice, unit: unit}, true
	case parts[1] == "adapter" && nested:
		return nextPackage{layer: nextAdapter, unit: unit}, true
	case parts[1] == "surface" && nested:
		return nextPackage{layer: nextSurface, unit: unit}, true
	}
	return nextPackage{}, false
}

func nextRoot(pass *analysis.Pass, rel string) string {
	dirPath := filepath.Dir(pass.Fset.File(firstFile(pass).Pos()).Name())
	return strings.TrimSuffix(strings.TrimSuffix(dirPath, filepath.FromSlash(rel)), string(filepath.Separator))
}

func fakesOf(root string) (map[string]bool, error) {
	// A fake is the adapter <name>test beside an adapter <name>: a "contest" with no "con" is an adapter.
	entries, err := os.ReadDir(filepath.Join(root, "internal", "adapter"))
	if os.IsNotExist(err) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	adapters := map[string]bool{}
	for _, entry := range entries {
		adapters[entry.Name()] = entry.IsDir()
	}
	fakes := map[string]bool{}
	for name := range adapters {
		subject, isTestName := strings.CutSuffix(name, "test")
		fakes[name] = isTestName && adapters[subject]
	}
	return fakes, nil
}

type nextImport struct {
	From   nextPackage
	Target string
	Fakes  map[string]bool
}

func nextRefuses(imp nextImport) (string, bool) {
	if strings.HasPrefix(imp.Target, internalPrefix) {
		return "next/ never imports internal/: copy what it needs and give it a home", true
	}
	rel, inNext := cutNext(imp.Target)
	to, inRow := nextPackageOf(rel)
	inNext = inNext && inRow
	fake := inNext && to.layer == nextAdapter && imp.Fakes[to.unit]
	if fake && !(imp.From.layer == nextAdapter && imp.From.unit == to.unit) {
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
	case nextSlice:
		return stdlib || in(nextDomain) || in(nextSlice) && (to.pkg.unit == from.unit || to.pkg.unit == sharedSlice)
	case nextApp:
		return stdlib || in(nextDomain, nextSlice)
	case nextAdapter:
		return stdlib || thirdParty || in(nextDomain) || in(nextAdapter) && admitsAdapter(imp, to.pkg)
	case nextSurface:
		return stdlib || thirdParty || in(nextDomain, nextApp, nextSlice, nextSurface)
	case nextCmd:
		return stdlib || thirdParty || to.inNext
	}
	return false
}

func admitsAdapter(imp nextImport, to nextPackage) bool {
	from := imp.From
	return to.unit == from.unit || slices.Contains(sharedAdapters, to.unit) || imp.Fakes[from.unit] && from.unit == to.unit+"test"
}

type nextCommentsParams struct {
	Tests    testFiles
	Ceilings func() (map[string]int, error)
}

var nextCommentsAnalyzer = newNextCommentsAnalyzer(nextCommentsParams{Tests: testFilesOnDisk, Ceilings: sync.OnceValues(ceilingsOnDisk)})

func newNextCommentsAnalyzer(params nextCommentsParams) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "nextcomments",
		Doc:  "a package of next/ has a one-line package doc and, beside it and the bare directives, exactly the comment lines its ceiling in .next-comments allows, godoc included",
		Run:  func(pass *analysis.Pass) (any, error) { return runNextComments(pass, params) },
	}
}

func runNextComments(pass *analysis.Pass, params nextCommentsParams) (any, error) {
	rel, ok := nextRel(pass)
	if !ok {
		return nil, nil
	}
	ceilings, err := params.Ceilings()
	if err != nil {
		return nil, err
	}
	tests, err := params.Tests(pass)
	if err != nil {
		return nil, err
	}
	sources := slices.DeleteFunc(slices.Clone(pass.Files), func(file *ast.File) bool { return isTestFile(pass, file) })
	checkNextPackageDoc(pass, rel, sources)
	for _, file := range tests {
		checkTestHeader(pass, file)
		checkBanners(pass, file)
	}
	lines := 0
	for _, file := range append(sources, tests...) {
		lines += commentLines(pass.Fset, file, nextDirective)
	}
	ceiling := ceilings[rel]
	switch {
	case lines > ceiling:
		pass.Reportf(firstFile(pass).Package, "next/%s has %d comment lines, above its ceiling of %d in .next-comments: delete the comments that restate the code; one that earns its place raises the ceiling, in review", rel, lines, ceiling)
	case lines < ceiling:
		pass.Reportf(firstFile(pass).Package, "next/%s has %d comment lines, under its ceiling of %d in .next-comments: lower it, a ceiling only goes down", rel, lines, ceiling)
	}
	return nil, nil
}

func checkNextPackageDoc(pass *analysis.Pass, rel string, sources []*ast.File) {
	documented := slices.DeleteFunc(slices.Clone(sources), func(file *ast.File) bool { return file.Doc == nil })
	if len(documented) == 0 {
		pass.Reportf(firstFile(pass).Package, "next/%s has no package doc: say its one responsibility in one line", rel)
		return
	}
	for _, file := range documented {
		if span := lineSpan(pass.Fset, file.Doc); span > 1 {
			pass.Reportf(file.Doc.Pos(), "next/%s has a package doc of %d lines: say its one responsibility in one line, the rest goes to next/README.md", rel, span)
		}
	}
}

var nextDirectives = []*regexp.Regexp{
	regexp.MustCompile(`^//go:build \S.*$`),
	regexp.MustCompile(`^//go:embed \S.*$`),
	regexp.MustCompile(`^//go:generate \S.*$`),
	regexp.MustCompile(`^//go-sumtype:decl \w+$`),
	regexp.MustCompile(`^//nolint:[\w,-]+$`),
}

func nextDirective(comment *ast.Comment) bool {
	return slices.ContainsFunc(nextDirectives, func(form *regexp.Regexp) bool { return form.MatchString(comment.Text) })
}

func ceilingsOnDisk() (map[string]int, error) {
	data, err := os.ReadFile(".next-comments")
	if os.IsNotExist(err) {
		return map[string]int{}, nil
	}
	if err != nil {
		return nil, err
	}
	ceilings, err := parseNextCeilings(string(data))
	if err != nil {
		return nil, err
	}
	if stale := staleCeilings(staleCeilingsParams{Root: "next", Ceilings: ceilings}); len(stale) > 0 {
		return nil, fmt.Errorf(".next-comments: no package under next/ for %s — remove its line", strings.Join(stale, ", "))
	}
	return ceilings, nil
}

func parseNextCeilings(data string) (map[string]int, error) {
	ceilings := map[string]int{}
	for _, raw := range strings.Split(data, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf(".next-comments: %q is not `<package under next/> <comment lines>`", line)
		}
		count, err := strconv.Atoi(fields[1])
		if err != nil || count < 0 {
			return nil, fmt.Errorf(".next-comments: %q: the count must be a number of lines", line)
		}
		if _, seen := ceilings[fields[0]]; seen {
			return nil, fmt.Errorf(".next-comments: %s has two lines", fields[0])
		}
		ceilings[fields[0]] = count
	}
	return ceilings, nil
}

type staleCeilingsParams struct {
	Root     string
	Ceilings map[string]int
}

func staleCeilings(params staleCeilingsParams) []string {
	var stale []string
	for pkg := range params.Ceilings {
		matches, err := filepath.Glob(filepath.Join(params.Root, filepath.FromSlash(pkg), "*.go"))
		if err != nil || len(matches) == 0 {
			stale = append(stale, pkg)
		}
	}
	slices.Sort(stale)
	return stale
}
