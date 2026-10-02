package main

import (
	"slices"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

// The daemon outlives the command that started it and serves every worktree of
// every repository: it never runs git and never reads a repository's config,
// it is handed what it needs. service/events joins the list before it exists,
// so LUC-233 cannot make the broker schema-aware by accident.
var (
	daemonForbidden = []string{"service/worktree", "service/branch", "service/github", "service/events", "config"}
	daemonInfra     = map[string]bool{"GlobalDir": true}
)

var daemonblindAnalyzer = &analysis.Analyzer{
	Name: "daemonblind",
	Doc:  "service/process imports nothing that runs git and only allow-listed infra",
	Run:  runDaemonBlind,
}

func runDaemonBlind(pass *analysis.Pass) (any, error) {
	if !dir("service/process").holds(pass.Pkg.Path()) {
		return nil, nil
	}
	for _, file := range pass.Files {
		for _, imp := range file.Imports {
			target, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			if slices.ContainsFunc(daemonForbidden, func(d string) bool { return dir(d).holds(target) }) {
				pass.Reportf(imp.Pos(), "the daemon must not import %q: service/process is blind to git and to the repository's config", target)
			}
		}
	}
	for ident, obj := range pass.TypesInfo.Uses {
		if obj.Pkg() == nil || !dir("infra").holds(obj.Pkg().Path()) || !packageLevel(obj) || daemonInfra[obj.Name()] {
			continue
		}
		pass.Reportf(ident.Pos(), "the daemon calls infra.%s: only GlobalDir is allow-listed — the daemon never runs git", obj.Name())
	}
	return nil, nil
}
