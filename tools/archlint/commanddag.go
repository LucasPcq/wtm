package main

import "golang.org/x/tools/go/analysis"

var commandDAG = edgesTable{root: "command", edges: commandEdges, name: "commandEdges"}

var commanddagAnalyzer = &analysis.Analyzer{
	Name: "commanddag",
	Doc:  "a command package imports another only along a declared edge",
	Run:  commandDAG.run,
}
