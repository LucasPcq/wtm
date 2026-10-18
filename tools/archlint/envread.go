package main

import (
	"go/ast"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// The functions of the stdlib that read the process's environment, by package.
var envReaders = map[string][]string{
	"os":      {"Getenv", "LookupEnv", "Environ", "ExpandEnv", "UserHomeDir", "UserConfigDir", "UserCacheDir"},
	"syscall": {"Getenv", "Environ"},
}

// The edges that read the process's environment: the root reads it once (ui/cli.ReadEnv), the streams their terminal, the composition root $WTM_STATE_DIR, and an adapter what its own tool needs.
var envEdges = []string{"internal/ui/cli", "internal/ui/output", nextRoot, "internal/adapter/"}

var envreadAnalyzer = &analysis.Analyzer{
	Name: "envread",
	Doc:  "the stdlib functions that read the environment (os.Getenv, os.LookupEnv, os.Environ, os.ExpandEnv, os.UserHomeDir, os.UserConfigDir, os.UserCacheDir and syscall's Getenv, Environ) are called in next/ only from ui/cli, ui/output, cmd/wtm-next and adapter/*",
	Run:  runEnvread,
}

func runEnvread(pass *analysis.Pass) (any, error) {
	rel, ok := nextRel(pass)
	if !ok || readsEnv(rel) {
		return nil, nil
	}
	for _, file := range pass.Files {
		if isTestFile(pass, file) {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			obj := pass.TypesInfo.Uses[sel.Sel]
			if obj == nil || obj.Pkg() == nil || !slices.Contains(envReaders[obj.Pkg().Path()], obj.Name()) {
				return true
			}
			pass.Reportf(sel.Pos(), "next/%s reads the environment (%s.%s): only ui/cli, ui/output, cmd/wtm-next and an adapter read it, at the edge; take the value as an input (next/README.md)", rel, obj.Pkg().Name(), obj.Name())
			return true
		})
	}
	return nil, nil
}

func readsEnv(rel string) bool {
	return slices.ContainsFunc(envEdges, func(edge string) bool {
		if strings.HasSuffix(edge, "/") {
			return strings.HasPrefix(rel, edge)
		}
		return rel == edge || strings.HasPrefix(rel, edge+"/")
	})
}
