package main

import (
	"slices"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

var (
	daemonPackages  = []dir{"service/process", "service/proxy"}
	daemonForbidden = []string{"service/worktree", "service/branch", "service/github", "service/events", "config"}
	daemonInfra     = map[string]bool{"GlobalDir": true, "ReplaceFile": true, "ReplaceFileParams": true}
)

var daemonblindAnalyzer = &analysis.Analyzer{
	Name: "daemonblind",
	Doc:  "the daemon (service/process, service/proxy) imports nothing that runs git and only allow-listed infra",
	Run:  runDaemonBlind,
}

func runDaemonBlind(pass *analysis.Pass) (any, error) {
	if !slices.ContainsFunc(daemonPackages, func(d dir) bool { return d.holds(pass.Pkg.Path()) }) {
		return nil, nil
	}
	for _, file := range pass.Files {
		for _, imp := range file.Imports {
			target, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			if slices.ContainsFunc(daemonForbidden, func(d string) bool { return dir(d).holds(target) }) {
				pass.Reportf(imp.Pos(), "the daemon must not import %q: service/process and service/proxy are blind to git and to the repository's config", target)
			}
		}
	}
	for ident, obj := range pass.TypesInfo.Uses {
		if obj.Pkg() == nil || !dir("infra").holds(obj.Pkg().Path()) || !packageLevel(obj) || daemonInfra[obj.Name()] {
			continue
		}
		pass.Reportf(ident.Pos(), "the daemon calls infra.%s: only GlobalDir and ReplaceFile are allow-listed — the daemon never runs git", obj.Name())
	}
	return nil, nil
}
