package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var (
	commentedPackages = []dir{"kernel", "dispatch", "command", "query", "registry", "flow/legacy", "store", "infra/git", "infra/fs", "infra/proc", "surface/tui/form"}
	commentedSubjects = []dir{"rules", "domain"}
	bannerRuns        = []string{"----", "====", "****", "####", "~~~~", "____"}
)

var commentsAnalyzer = newCommentsAnalyzer(testFilesOnDisk)

type testFiles func(pass *analysis.Pass) ([]*ast.File, error)

func newCommentsAnalyzer(testsOf testFiles) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "comments",
		Doc:  "the engine's comments keep their shapes: a one-line package doc, no doc on an unexported declaration, no banner or header in a test, and no more comment lines than the package's ceiling",
		Run:  func(pass *analysis.Pass) (any, error) { return runComments(pass, testsOf) },
	}
}

func runComments(pass *analysis.Pass, testsOf testFiles) (any, error) {
	if !commented(pass.Pkg.Path()) || len(pass.Files) == 0 {
		return nil, nil
	}
	tests, err := testsOf(pass)
	if err != nil {
		return nil, err
	}
	sources := slices.DeleteFunc(slices.Clone(pass.Files), func(file *ast.File) bool { return isTestFile(pass, file) })
	for _, file := range sources {
		checkPackageDoc(pass, file)
		checkUnexportedDocs(pass, file)
	}
	for _, file := range tests {
		checkUnexportedDocs(pass, file)
		checkTestHeader(pass, file)
		checkBanners(pass, file)
	}
	checkCeiling(pass, append(sources, tests...))
	return nil, nil
}

func commented(pkgPath string) bool {
	if slices.ContainsFunc(commentedPackages, func(d dir) bool { return d.holds(pkgPath) }) {
		return true
	}
	if slices.ContainsFunc(engineServices, func(d dir) bool { return d.holds(pkgPath) }) {
		return true
	}
	return slices.ContainsFunc(commentedSubjects, func(d dir) bool { return d.holds(pkgPath) && internalPath(pkgPath) != string(d) })
}

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

func checkPackageDoc(pass *analysis.Pass, file *ast.File) {
	if file.Doc == nil || lineSpan(pass.Fset, file.Doc) == 1 {
		return
	}
	pass.Reportf(file.Doc.Pos(), "package %s has a package doc of %d lines: say its one responsibility in one line, the rest goes to docs/dev/ (CLAUDE.md §8, comments)", internalPath(pass.Pkg.Path()), lineSpan(pass.Fset, file.Doc))
}

func checkUnexportedDocs(pass *analysis.Pass, file *ast.File) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			reportUnexportedDoc(pass, d.Doc, d.Name)
		case *ast.GenDecl:
			checkGenDecl(pass, d)
		}
	}
}

func checkGenDecl(pass *analysis.Pass, decl *ast.GenDecl) {
	if decl.Tok == token.IMPORT {
		return
	}
	names := declaredNames(decl.Specs)
	if !slices.ContainsFunc(names, (*ast.Ident).IsExported) && len(names) > 0 {
		reportUnexportedDoc(pass, decl.Doc, names[0])
	}
	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			reportUnexportedDoc(pass, s.Doc, s.Name)
		case *ast.ValueSpec:
			if !slices.ContainsFunc(s.Names, (*ast.Ident).IsExported) {
				reportUnexportedDoc(pass, s.Doc, s.Names[0])
			}
		}
	}
}

func declaredNames(specs []ast.Spec) []*ast.Ident {
	var names []*ast.Ident
	for _, spec := range specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			names = append(names, s.Name)
		case *ast.ValueSpec:
			names = append(names, s.Names...)
		}
	}
	return names
}

func reportUnexportedDoc(pass *analysis.Pass, doc *ast.CommentGroup, name *ast.Ident) {
	if doc == nil || name.IsExported() || directivesOnly(doc) {
		return
	}
	pass.Reportf(doc.Pos(), "doc comment on the unexported %s: a name says what it is; a why goes in the body, beside the line it explains (CLAUDE.md §8, comments)", name.Name)
}

func checkTestHeader(pass *analysis.Pass, file *ast.File) {
	for _, group := range file.Comments {
		if group.End() < file.Package && !directivesOnly(group) {
			pass.Reportf(group.Pos(), "comment above the package clause of a test file: a test file has no header, the package doc says what the package is (CLAUDE.md §8, comments)")
		}
	}
}

func checkBanners(pass *analysis.Pass, file *ast.File) {
	for _, group := range file.Comments {
		if group.Pos() < file.Package || directivesOnly(group) {
			continue
		}
		if floating(file, group) || slices.ContainsFunc(group.List, ruled) {
			pass.Reportf(group.Pos(), "section banner in a test file: a test is found by its name, a group by its file (CLAUDE.md §8, comments)")
		}
	}
}

func floating(file *ast.File, group *ast.CommentGroup) bool {
	return !slices.ContainsFunc(file.Decls, func(decl ast.Decl) bool {
		return group.Pos() >= decl.Pos() && group.End() <= decl.End() || docOf(decl) == group
	})
}

func docOf(decl ast.Decl) *ast.CommentGroup {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return d.Doc
	case *ast.GenDecl:
		return d.Doc
	}
	return nil
}

func ruled(comment *ast.Comment) bool {
	return slices.ContainsFunc(bannerRuns, func(run string) bool { return strings.Contains(comment.Text, run) })
}

func checkCeiling(pass *analysis.Pass, files []*ast.File) {
	pkg := internalPath(pass.Pkg.Path())
	lines := 0
	for _, file := range files {
		lines += commentLines(pass.Fset, file)
	}
	ceiling := commentCeiling[pkg]
	if lines <= ceiling {
		return
	}
	pass.Reportf(firstFile(pass).Package, "package %s has %d comment lines, above its ceiling of %d: delete the comments that restate the code; one that earns its place raises commentCeiling (tools/archlint/ceiling.go), in review (CLAUDE.md §8, comments)", pkg, lines, ceiling)
}

func commentLines(fset *token.FileSet, file *ast.File) int {
	lines := map[int]bool{}
	for _, group := range file.Comments {
		if group == file.Doc {
			continue
		}
		for _, comment := range group.List {
			if directive(comment) {
				continue
			}
			for line := fset.Position(comment.Pos()).Line; line <= fset.Position(comment.End()).Line; line++ {
				lines[line] = true
			}
		}
	}
	return len(lines)
}

func lineSpan(fset *token.FileSet, group *ast.CommentGroup) int {
	return fset.Position(group.End()).Line - fset.Position(group.Pos()).Line + 1
}

func directivesOnly(group *ast.CommentGroup) bool {
	return !slices.ContainsFunc(group.List, func(comment *ast.Comment) bool { return !directive(comment) })
}

func directive(comment *ast.Comment) bool {
	return strings.HasPrefix(comment.Text, "//go:") || strings.HasPrefix(comment.Text, "//go-sumtype:") || strings.HasPrefix(comment.Text, "//nolint")
}
