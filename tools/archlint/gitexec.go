package main

import (
	"go/ast"
	"go/constant"
	"slices"

	"golang.org/x/tools/go/analysis"
)

// processStarts are the calls that start a program by name. Given "git", they
// bypass infra/git: its one parser per output, its remote handed in, its
// errors returned rather than read as a default.
var processStarts = []objectRef{
	{Path: "os/exec", Name: "Command"},
	{Path: "os/exec", Name: "CommandContext"},
	{Path: internalPrefix + "infra/proc", Name: "Command"},
	{Path: internalPrefix + "infra/proc", Name: "GroupCommand"},
}

// gitExecExempt are the packages that run git for good outside infra/git.
var gitExecExempt = map[string]string{
	"testutil": "test helpers build real repositories with git itself",
}

const gitProgram = "git"

var gitexecAnalyzer = &analysis.Analyzer{
	Name: "gitexec",
	Doc:  "git is run only by infra/git",
	Run:  runGitexec,
}

func runGitexec(pass *analysis.Pass) (any, error) {
	if dir("infra/git").holds(pass.Pkg.Path()) || exemptFromGitexec(pass.Pkg.Path()) {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if ok && startsGit(pass, call) {
				pass.Reportf(call.Pos(), "git run outside infra/git: call it through git.Repo (docs/dev/git.md, gitexec)")
			}
			return true
		})
	}
	return nil, nil
}

func startsGit(pass *analysis.Pass, call *ast.CallExpr) bool {
	obj := objectOf(pass.TypesInfo, call.Fun)
	if !slices.ContainsFunc(processStarts, func(ref objectRef) bool { return refers(obj, ref) }) {
		return false
	}
	return slices.ContainsFunc(call.Args, func(arg ast.Expr) bool {
		value := pass.TypesInfo.Types[arg].Value
		return value != nil && value.Kind() == constant.String && constant.StringVal(value) == gitProgram
	})
}

func exemptFromGitexec(pkgPath string) bool {
	for key := range gitExecExempt {
		if dir(key).holds(pkgPath) {
			return true
		}
	}
	return false
}
