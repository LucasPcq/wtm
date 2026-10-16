package main

import (
	"fmt"
	"go/ast"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
)

const nextPrefix = modulePath + "next/"

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

type nextPackage struct {
	path  string
	layer nextLayer
	unit  string
}

var nextWhy = map[nextLayer]string{
	nextDomain:  "the domain is pure: the stdlib and wtm/* only",
	nextSlice:   "a use case reaches the world through the interfaces it declares: wtm/*, app/provision and the stdlib, never another slice, an adapter, a surface or app",
	nextApp:     "the API every surface calls: app/*, wtm/* and the stdlib, never an adapter",
	nextAdapter: "an adapter reaches wtm/*, adapter/statefile, the stdlib and third parties, never app/*, a surface or another adapter",
	nextSurface: "a surface calls the app: app, app/<slice>, wtm/*, surface/* and third parties, never an adapter",
	nextCmd:     "the composition root wires what is under next/",
}

var nextAnalyzer = &analysis.Analyzer{
	Name: "next",
	Doc:  "a package of next/ imports only what its row of next/README.md allows, never internal/, and a fake only from a test",
	Run:  runNext,
}

func runNext(pass *analysis.Pass) (any, error) {
	rel, ok := nextRel(pass)
	if !ok {
		return nil, nil
	}
	from, ok := nextPackageOf(pass.Pkg.Path())
	if !ok {
		pass.Reportf(firstFile(pass).Package, "next/%s is in no row of next/'s import table: place it under wtm/, app/, adapter/, surface/ or cmd/ (next/README.md)", rel)
		return nil, nil
	}
	for _, file := range pass.Files {
		if isTestFile(pass, file) {
			continue
		}
		for _, spec := range file.Imports {
			target, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return nil, err
			}
			if why, refused := nextRefuses(from, target); refused {
				pass.Reportf(spec.Pos(), "next/%s must not import %q — %s (next/README.md)", rel, target, why)
			}
		}
	}
	return nil, nil
}

// A test binary's generated main package is not code anyone wrote.
func nextRel(pass *analysis.Pass) (string, bool) {
	rel, ok := strings.CutPrefix(pass.Pkg.Path(), nextPrefix)
	return rel, ok && len(pass.Files) > 0 && !strings.HasSuffix(rel, ".test")
}

func nextPackageOf(pkgPath string) (nextPackage, bool) {
	rel, ok := strings.CutPrefix(pkgPath, nextPrefix)
	if !ok {
		return nextPackage{}, false
	}
	parts := strings.Split(rel, "/")
	unit := func() string {
		if len(parts) < 3 {
			return ""
		}
		return parts[2]
	}
	switch {
	case parts[0] == "cmd":
		return nextPackage{path: rel, layer: nextCmd}, true
	case len(parts) < 2 || parts[0] != "internal":
		return nextPackage{}, false
	case parts[1] == "wtm":
		return nextPackage{path: rel, layer: nextDomain}, true
	case parts[1] == "app" && len(parts) == 2:
		return nextPackage{path: rel, layer: nextApp}, true
	case parts[1] == "app":
		return nextPackage{path: rel, layer: nextSlice, unit: unit()}, true
	case parts[1] == "adapter" && len(parts) > 2:
		return nextPackage{path: rel, layer: nextAdapter, unit: unit()}, true
	case parts[1] == "surface" && len(parts) > 2:
		return nextPackage{path: rel, layer: nextSurface, unit: unit()}, true
	}
	return nextPackage{}, false
}

func nextRefuses(from nextPackage, target string) (string, bool) {
	if strings.HasPrefix(target, internalPrefix) {
		return "next/ never imports internal/: copy what it needs and give it a home", true
	}
	to, inNext := nextPackageOf(target)
	if inNext && isFake(to) && !(from.layer == nextAdapter && from.unit == to.unit) {
		return "a fake is imported from a _test.go only", true
	}
	if nextAdmits(from, nextTarget{path: target, pkg: to, inNext: inNext}) {
		return "", false
	}
	return nextWhy[from.layer], true
}

type nextTarget struct {
	path   string
	pkg    nextPackage
	inNext bool
}

func nextAdmits(from nextPackage, to nextTarget) bool {
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
		return stdlib || thirdParty || in(nextDomain) || in(nextAdapter) && admitsAdapter(from, to.pkg)
	case nextSurface:
		return stdlib || thirdParty || in(nextDomain, nextApp, nextSlice, nextSurface)
	case nextCmd:
		return stdlib || thirdParty || to.inNext
	}
	return false
}

func admitsAdapter(from, to nextPackage) bool {
	return to.unit == from.unit || to.unit == "statefile" || from.unit == to.unit+"test"
}

func isFake(pkg nextPackage) bool {
	return pkg.layer == nextAdapter && strings.HasSuffix(pkg.unit, "test")
}

type nextCommentsParams struct {
	Tests    testFiles
	Ceilings func() (map[string]int, error)
}

var nextCommentsAnalyzer = newNextCommentsAnalyzer(nextCommentsParams{Tests: testFilesOnDisk, Ceilings: sync.OnceValues(ceilingsOnDisk)})

func newNextCommentsAnalyzer(params nextCommentsParams) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "nextcomments",
		Doc:  "a package of next/ has a one-line package doc and, beside it and the godoc of its exported symbols, exactly the comment lines its ceiling in .next-comments allows",
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
		lines += commentLines(pass.Fset, file, godoc(file))
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

func godoc(file *ast.File) map[*ast.CommentGroup]bool {
	docs := map[*ast.CommentGroup]bool{file.Doc: true}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			docs[d.Doc] = d.Name.IsExported()
		case *ast.GenDecl:
			docs[d.Doc] = slices.ContainsFunc(declaredNames(d.Specs), (*ast.Ident).IsExported)
			for _, spec := range d.Specs {
				addSpecDoc(docs, spec)
			}
		}
	}
	return docs
}

func addSpecDoc(docs map[*ast.CommentGroup]bool, spec ast.Spec) {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		docs[s.Doc] = s.Name.IsExported()
	case *ast.ValueSpec:
		docs[s.Doc] = slices.ContainsFunc(s.Names, (*ast.Ident).IsExported)
	}
}

func ceilingsOnDisk() (map[string]int, error) {
	data, err := os.ReadFile(".next-comments")
	if os.IsNotExist(err) {
		return map[string]int{}, nil
	}
	if err != nil {
		return nil, err
	}
	return parseNextCeilings(string(data))
}

func parseNextCeilings(data string) (map[string]int, error) {
	ceilings := map[string]int{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
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
		ceilings[fields[0]] = count
	}
	return ceilings, nil
}
