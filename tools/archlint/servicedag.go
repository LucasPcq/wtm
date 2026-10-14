package main

import (
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type edgesTable struct {
	root  dir
	edges map[string][]string
	name  string
}

var serviceDAG = edgesTable{root: "service", edges: serviceEdges, name: "serviceEdges"}

var servicedagAnalyzer = &analysis.Analyzer{
	Name: "servicedag",
	Doc:  "a service package imports another only along a declared edge",
	Run:  serviceDAG.run,
}

func (table edgesTable) run(pass *analysis.Pass) (any, error) {
	own := table.top(pass.Pkg.Path())
	if own == "" {
		return nil, nil
	}
	for _, file := range pass.Files {
		for _, imp := range file.Imports {
			target, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			other := table.top(target)
			if other == "" || other == own || slices.Contains(table.edges[own], other) {
				continue
			}
			pass.Reportf(imp.Pos(), "internal/%s/%s must not import %q — undeclared edge: declare it in %s (tools/archlint/layers.go), see CLAUDE.md section 9", table.root, own, target, table.name)
		}
	}
	return nil, nil
}

// top is "process" for service/process/processtest, "" outside its root.
func (table edgesTable) top(pkgPath string) string {
	if !table.root.holds(pkgPath) {
		return ""
	}
	parts := strings.Split(internalPath(pkgPath), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}
