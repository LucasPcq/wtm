package main

import (
	"go/ast"
	"go/types"
	"path/filepath"

	"golang.org/x/tools/go/analysis"
)

var (
	cobraCommand    = objectRef{Path: "github.com/spf13/cobra", Name: "Command"}
	interactiveGate = objectRef{Path: internalPrefix + "commands/shared", Name: "Interactive"}
	yesFlag         = objectRef{Path: internalPrefix + "commands/shared", Name: "AddYesFlag"}
)

// yesflagAnalyzer enforces the confirmation axis: a file that builds a command
// and reads the prompt-capability gate registers --yes on it. A helper that
// merely takes a command has no flags of its own to register.
var yesflagAnalyzer = &analysis.Analyzer{
	Name: "yesflag",
	Doc:  "a command reading the interactive gate registers --yes",
	Run:  runYesFlag,
}

func runYesFlag(pass *analysis.Pass) (any, error) {
	if layerOfPackage(pass.Pkg.Path()) != "commands" {
		return nil, nil
	}
	for _, file := range pass.Files {
		used := referencedIn(pass.TypesInfo, file)
		if !used[cobraCommand] || !used[interactiveGate] || used[yesFlag] || !buildsCommand(pass.TypesInfo, file) {
			continue
		}
		name := filepath.Base(pass.Fset.Position(file.Pos()).Filename)
		pass.Reportf(file.Pos(), "%s reads the interactive gate but registers no --yes: the confirmation axis is a flag, not an inference", name)
	}
	return nil, nil
}

func referencedIn(info *types.Info, file *ast.File) map[objectRef]bool {
	seen := map[objectRef]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		obj := info.Uses[ident]
		if obj == nil || obj.Pkg() == nil {
			return true
		}
		seen[objectRef{Path: obj.Pkg().Path(), Name: obj.Name()}] = true
		return true
	})
	return seen
}

func buildsCommand(info *types.Info, file *ast.File) bool {
	built := false
	ast.Inspect(file, func(node ast.Node) bool {
		if lit, ok := node.(*ast.CompositeLit); ok && refers(objectOf(info, lit.Type), cobraCommand) {
			built = true
		}
		return !built
	})
	return built
}
