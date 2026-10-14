package main

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"

	"golang.org/x/tools/go/analysis"
)

// fileWrites are the writes that bypass infra/fs: a file truncated in place
// by os.WriteFile or os.Create is half a file to whoever reads it meanwhile,
// a hand-rolled rename or removal takes no lock.
var fileWrites = []objectRef{
	{Path: "os", Name: "WriteFile"},
	{Path: "os", Name: "Create"},
	{Path: "os", Name: "Rename"},
	{Path: "os", Name: "Remove"},
	{Path: "os", Name: "RemoveAll"},
	{Path: "os", Name: "OpenFile"},
}

var (
	openFile = objectRef{Path: "os", Name: "OpenFile"}
	// openWriteFlags are the os.OpenFile flags that make it a write; a flag
	// argument the checker cannot fold counts as one.
	openWriteFlags = []string{"O_WRONLY", "O_RDWR", "O_CREATE", "O_APPEND", "O_TRUNC"}
)

// stateWriteExempt are the writers that stay outside infra/fs for good, keyed
// by package and, when only one file of it is exempt, by that file.
var stateWriteExempt = map[string]string{
	"service/process/logstore.go":   "log rotation renames and appends job logs, which are streams, not state",
	"service/process/daemon.go":     "the daemon clears its own stale socket, which is not a file",
	"service/selfupdate/replace.go": "replacing the wtm binary is not a state file",
	"surface/cli/agents/install.go": "agent skills are written into the user's or the project's agent directories",
	"surface/cli/schema/dump.go":    "`wtm schema dump` writes into a directory the user names",
	"service/shell":                 "the cd handshake writes the file the shell wrapper reads",
	"service/proxy":                 "the launchd plist belongs to the system, not to wtm's state",
	"service/process/processtest":   "the daemon's test double clears the sockets its tests opened",
	"testutil":                      "fixtures written by test helpers",
}

var statewriteAnalyzer = &analysis.Analyzer{
	Name: "statewrite",
	Doc:  "os.WriteFile, Create, Rename, Remove, RemoveAll and a writing os.OpenFile only inside infra/: every other write goes through infra/fs",
	Run:  runStatewrite,
}

func runStatewrite(pass *analysis.Pass) (any, error) {
	if dir("infra").holds(pass.Pkg.Path()) {
		return nil, nil
	}
	pkg := internalPath(pass.Pkg.Path())
	reads := readOnlyOpens(pass)
	for ident, obj := range pass.TypesInfo.Uses {
		if !writesAFile(obj) || reads[ident] {
			continue
		}
		file := filepath.Base(pass.Fset.Position(ident.Pos()).Filename)
		if exemptFromStatewrite(pkg, file) {
			continue
		}
		pass.Reportf(ident.Pos(), "os.%s outside infra/: write through infra/fs (docs/dev/state-files.md, statewrite)", obj.Name())
	}
	return nil, nil
}

func writesAFile(obj types.Object) bool {
	return slices.ContainsFunc(fileWrites, func(ref objectRef) bool { return refers(obj, ref) })
}

// readOnlyOpens are the os.OpenFile calls whose flags fold to a constant with
// no write bit: they read, whatever their spelling.
func readOnlyOpens(pass *analysis.Pass) map[*ast.Ident]bool {
	reads := map[*ast.Ident]bool{}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			fun, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !refers(pass.TypesInfo.Uses[fun.Sel], openFile) {
				return true
			}
			if flags := pass.TypesInfo.Types[call.Args[1]].Value; flags != nil && !opensForWriting(pass.TypesInfo.Uses[fun.Sel].Pkg(), flags) {
				reads[fun.Sel] = true
			}
			return true
		})
	}
	return reads
}

func opensForWriting(osPkg *types.Package, flags constant.Value) bool {
	for _, name := range openWriteFlags {
		bit, ok := osPkg.Scope().Lookup(name).(*types.Const)
		if !ok {
			continue
		}
		if constant.Compare(constant.BinaryOp(flags, token.AND, bit.Val()), token.NEQ, constant.MakeInt64(0)) {
			return true
		}
	}
	return false
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
