package main

import (
	"slices"

	"golang.org/x/tools/go/analysis"
)

// stateWrites are the writes of a file wtm keeps. Called from anywhere, a state
// file has as many owners as callers, each with its own idea of its schema;
// store/ is its one owner, behind an interface a fake doubles.
var stateWrites = []objectRef{
	{Path: internalPrefix + "infra/fs", Name: "UpdateFile"},
	{Path: internalPrefix + "infra/fs", Name: "RemoveDir"},
}

var stateownerAnalyzer = &analysis.Analyzer{
	Name: "stateowner",
	Doc:  "fs.UpdateFile and fs.RemoveDir are called from store/ only: each state file has one owner",
	Run:  runStateowner,
}

func runStateowner(pass *analysis.Pass) (any, error) {
	if dir("store").holds(pass.Pkg.Path()) || dir("infra/fs").holds(pass.Pkg.Path()) {
		return nil, nil
	}
	for ident, obj := range pass.TypesInfo.Uses {
		if !slices.ContainsFunc(stateWrites, func(ref objectRef) bool { return refers(obj, ref) }) {
			continue
		}
		pass.Reportf(ident.Pos(), "fs.%s outside store/: a state file is written by its owner in store/ (docs/dev/store.md, stateowner)", obj.Name())
	}
	return nil, nil
}
