package main

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var processStarts = []processStart{
	{ref: objectRef{Path: "os/exec", Name: "Command"}, program: 0},
	{ref: objectRef{Path: "os/exec", Name: "CommandContext"}, program: 1},
	{ref: objectRef{Path: internalPrefix + "infra/proc", Name: "Command"}, program: 1},
	{ref: objectRef{Path: internalPrefix + "infra/proc", Name: "GroupCommand"}, program: 1},
}

type processStart struct {
	ref     objectRef
	program int // the index of the argument naming the program
}

var gitExecExempt = map[string]string{
	"testutil": "test helpers build real repositories with git itself",
}

const gitProgram = "git"

type programFact struct{ Index int }

func (*programFact) AFact() {}

func (f *programFact) String() string {
	return fmt.Sprintf("starts the program its parameter %d names", f.Index)
}

var gitexecAnalyzer = &analysis.Analyzer{
	Name:      "gitexec",
	Doc:       "git is run only by infra/git",
	Run:       runGitexec,
	FactTypes: []analysis.Fact{new(programFact)},
}

func runGitexec(pass *analysis.Pass) (any, error) {
	check := gitCheck{pass: pass, wrappers: map[*types.Func]int{}, gitVars: gitVarsOf(pass)}
	check.findWrappers()
	for fn, index := range check.wrappers {
		pass.ExportObjectFact(fn, &programFact{Index: index})
	}
	if dir("infra/git").holds(pass.Pkg.Path()) || exemptFromGitexec(pass.Pkg.Path()) {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if ok && slices.ContainsFunc(check.gitArgs(call), check.isGit) {
				pass.Reportf(call.Pos(), "git run outside infra/git: call it through git.Repo (docs/dev/git.md, gitexec)")
			}
			return true
		})
	}
	return nil, nil
}

type gitCheck struct {
	pass     *analysis.Pass
	wrappers map[*types.Func]int
	gitVars  map[*types.Var]bool
}

// Any string a start is given may say "git" (`env git …`).
func (c gitCheck) gitArgs(call *ast.CallExpr) []ast.Expr {
	if _, ok := c.startOf(call); ok {
		return slices.DeleteFunc(slices.Clone(call.Args), func(arg ast.Expr) bool { return !isString(c.pass.TypesInfo.TypeOf(arg)) })
	}
	return c.programArg(call)
}

func (c gitCheck) programArg(call *ast.CallExpr) []ast.Expr {
	index, ok := c.startOf(call)
	if !ok {
		fn, isFunc := objectOf(c.pass.TypesInfo, ast.Unparen(call.Fun)).(*types.Func)
		if !isFunc {
			return nil
		}
		index, ok = c.wrapperIndex(fn.Origin())
	}
	if !ok || index >= len(call.Args) {
		return nil
	}
	return call.Args[index : index+1]
}

func (c gitCheck) startOf(call *ast.CallExpr) (int, bool) {
	obj := objectOf(c.pass.TypesInfo, ast.Unparen(call.Fun))
	for _, start := range processStarts {
		if refers(obj, start.ref) {
			return start.program, true
		}
	}
	return 0, false
}

func (c gitCheck) wrapperIndex(fn *types.Func) (int, bool) {
	if index, ok := c.wrappers[fn]; ok {
		return index, true
	}
	fact := new(programFact)
	if fn.Pkg() != c.pass.Pkg && c.pass.ImportObjectFact(fn, fact) {
		return fact.Index, true
	}
	return 0, false
}

func isString(typ types.Type) bool {
	basic, ok := typ.(*types.Basic)
	return ok && basic.Info()&types.IsString != 0
}

func (c gitCheck) isGit(arg ast.Expr) bool {
	value := c.pass.TypesInfo.Types[arg].Value
	if value != nil {
		return value.Kind() == constant.String && constant.StringVal(value) == gitProgram
	}
	ident, ok := ast.Unparen(arg).(*ast.Ident)
	if !ok {
		return false
	}
	variable, ok := c.pass.TypesInfo.Uses[ident].(*types.Var)
	return ok && c.gitVars[variable]
}

func (c gitCheck) findWrappers() {
	for changed := true; changed; {
		changed = false
		for _, file := range c.pass.Files {
			for _, d := range file.Decls {
				decl, ok := d.(*ast.FuncDecl)
				if !ok || decl.Body == nil {
					continue
				}
				fn, ok := c.pass.TypesInfo.Defs[decl.Name].(*types.Func)
				if !ok {
					continue
				}
				if _, known := c.wrappers[fn]; known {
					continue
				}
				if index, ok := c.passesParam(decl); ok {
					c.wrappers[fn] = index
					changed = true
				}
			}
		}
	}
}

func (c gitCheck) passesParam(decl *ast.FuncDecl) (int, bool) {
	params := paramsOf(c.pass, decl)
	found, at := false, 0
	ast.Inspect(decl.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || found {
			return !found
		}
		for _, arg := range c.programArg(call) {
			ident, ok := ast.Unparen(arg).(*ast.Ident)
			if !ok {
				continue
			}
			if index := slices.Index(params, c.pass.TypesInfo.Uses[ident]); index >= 0 {
				found, at = true, index
				return false
			}
		}
		return true
	})
	return at, found
}

func paramsOf(pass *analysis.Pass, decl *ast.FuncDecl) []types.Object {
	var params []types.Object
	for _, field := range decl.Type.Params.List {
		for _, name := range field.Names {
			params = append(params, pass.TypesInfo.Defs[name])
		}
	}
	return params
}

func gitVarsOf(pass *analysis.Pass) map[*types.Var]bool {
	vars := map[*types.Var]bool{}
	mark := func(name *ast.Ident, value ast.Expr) {
		tv := pass.TypesInfo.Types[value]
		variable, ok := pass.TypesInfo.Defs[name].(*types.Var)
		if ok && tv.Value != nil && tv.Value.Kind() == constant.String && constant.StringVal(tv.Value) == gitProgram {
			vars[variable] = true
		}
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.ValueSpec:
				for index, name := range n.Names {
					if index < len(n.Values) {
						mark(name, n.Values[index])
					}
				}
			case *ast.AssignStmt:
				if n.Tok != token.DEFINE || len(n.Lhs) != len(n.Rhs) {
					return true
				}
				for index, lhs := range n.Lhs {
					if name, ok := lhs.(*ast.Ident); ok {
						mark(name, n.Rhs[index])
					}
				}
			}
			return true
		})
	}
	return vars
}

func exemptFromGitexec(pkgPath string) bool {
	for key := range gitExecExempt {
		if dir(key).holds(pkgPath) {
			return true
		}
	}
	return false
}
