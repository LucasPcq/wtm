package main

import (
	"go/types"
	"path/filepath"
	"slices"

	"golang.org/x/tools/go/analysis"
)

// fileWrites are the writes that bypass infra/fs: a file truncated in place
// by os.WriteFile or os.Create is half a file to whoever reads it meanwhile,
// and a hand-rolled rename takes no lock.
var fileWrites = []objectRef{
	{Path: "os", Name: "WriteFile"},
	{Path: "os", Name: "Create"},
	{Path: "os", Name: "Rename"},
}

// stateWriteExempt are the writers that stay outside infra/fs for good, keyed
// by package and, when only one file of it is exempt, by that file.
var stateWriteExempt = map[string]string{
	"service/process/logstore.go":   "log rotation renames and appends job logs, which are streams, not state",
	"service/selfupdate/replace.go": "replacing the wtm binary is not a state file",
	"surface/cli/agents/install.go": "agent skills are written into the user's or the project's agent directories",
	"surface/cli/schema/dump.go":    "`wtm schema dump` writes into a directory the user names",
	"service/shell":                 "the cd handshake writes the file the shell wrapper reads",
	"service/proxy":                 "the launchd plist belongs to the system, not to wtm's state",
	"testutil":                      "fixtures written by test helpers",
}

var statewriteAnalyzer = &analysis.Analyzer{
	Name: "statewrite",
	Doc:  "os.WriteFile, os.Create and os.Rename only inside infra/: every other write goes through infra/fs",
	Run:  runStatewrite,
}

func runStatewrite(pass *analysis.Pass) (any, error) {
	if dir("infra").holds(pass.Pkg.Path()) {
		return nil, nil
	}
	pkg := internalPath(pass.Pkg.Path())
	for ident, obj := range pass.TypesInfo.Uses {
		if !writesAFile(obj) {
			continue
		}
		file := filepath.Base(pass.Fset.Position(ident.Pos()).Filename)
		if exemptFromStatewrite(pkg, file) {
			continue
		}
		pass.Reportf(ident.Pos(), "os.%s outside infra/: write through fs.UpdateFile (docs/dev/lint.md, statewrite)", obj.Name())
	}
	return nil, nil
}

func writesAFile(obj types.Object) bool {
	return slices.ContainsFunc(fileWrites, func(ref objectRef) bool { return refers(obj, ref) })
}

func exemptFromStatewrite(pkg, file string) bool {
	if _, ok := stateWriteExempt[pkg+"/"+file]; ok {
		return true
	}
	for key := range stateWriteExempt {
		if dir(key).holds(internalPrefix + pkg) {
			return true
		}
	}
	return false
}
