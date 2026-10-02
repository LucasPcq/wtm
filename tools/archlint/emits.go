package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// emitters are the functions through which a flow publishes each event. A flow
// that calls a mutator and none of these has changed a worktree no consumer
// will hear about.
var emitters = map[string][]objectRef{
	"worktree.created":    {{Path: internalPrefix + "flow/publish", Name: "Created"}},
	"worktree.removed":    {{Path: internalPrefix + "flow/publish", Name: "Removed"}, {Path: internalPrefix + "flow/teardown", Name: "PublishRemoved"}},
	"worktree.relocated":  {{Path: internalPrefix + "flow/publish", Name: "Relocated"}},
	"worktree.reparented": {{Path: internalPrefix + "flow/publish", Name: "Reparented"}, {Path: internalPrefix + "flow/publish", Name: "ReparentedAll"}},
	"worktree.updated":    {{Path: internalPrefix + "flow/publish", Name: "Updated"}, {Path: internalPrefix + "flow/ordinal", Name: "Ensure"}},
}

const flowtestPath = internalPrefix + "testutil/flowtest"

var emitsAnalyzer = &analysis.Analyzer{
	Name: "emits",
	Doc:  "a flow package that calls a mutator publishes its event, and has a test recording what it publishes",
	Run:  runEmits,
}

func runEmits(pass *analysis.Pass) (any, error) {
	if !dir("flow").holds(pass.Pkg.Path()) || len(pass.Files) == 0 {
		return nil, nil
	}
	published := publishedEvents(pass)
	tested := recordsPublications(pass)
	for _, file := range pass.Files {
		if strings.HasSuffix(pass.Fset.File(file.Pos()).Name(), "_test.go") {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
			if !ok {
				return true
			}
			event := eventOf(fn)
			if event == "" {
				return true
			}
			if !published[event] {
				pass.Reportf(sel.Pos(), "%s.%s changes a worktree's identity but this package never publishes %s: call the flow/publish function for it", qualifierOf(sel, fn), fn.Name(), event)
				return true
			}
			if !tested {
				pass.Reportf(sel.Pos(), "%s.%s changes a worktree's identity but no test of this package records what it publishes (flowtest.Recorder as the Context's Publisher)", qualifierOf(sel, fn), fn.Name())
			}
			return true
		})
	}
	return nil, nil
}

func eventOf(fn *types.Func) string {
	if !isMutator(fn) {
		return ""
	}
	pkg := internalPath(fn.Pkg().Path())
	for _, m := range mutators {
		if m.pkg == pkg && m.name == fn.Name() {
			return m.event
		}
	}
	return ""
}

func publishedEvents(pass *analysis.Pass) map[string]bool {
	published := map[string]bool{}
	for _, obj := range pass.TypesInfo.Uses {
		for event, refs := range emitters {
			for _, ref := range refs {
				if refers(obj, ref) {
					published[event] = true
				}
			}
		}
	}
	return published
}

// recordsPublications reads the package's test files from disk: archlint loads
// the packages without their tests, and the rule is about whether one exists.
func recordsPublications(pass *analysis.Pass) bool {
	dirPath := filepath.Dir(pass.Fset.File(pass.Files[0].Pos()).Name())
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dirPath, entry.Name()), nil, 0)
		if err == nil && mentionsRecorder(file) {
			return true
		}
	}
	return false
}

func mentionsRecorder(file *ast.File) bool {
	name := ""
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != flowtestPath {
			continue
		}
		name = "flowtest"
		if imp.Name != nil {
			name = imp.Name.Name
		}
	}
	if name == "" {
		return false
	}
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return !found
		}
		if x, ok := sel.X.(*ast.Ident); ok && x.Name == name && sel.Sel.Name == "Recorder" {
			found = true
		}
		return !found
	})
	return found
}
