package main

import (
	"go/types"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestAMutatorIsReachedOnlyFromFlow(t *testing.T) {
	runTxtar(t, chokepointAnalyzer, "chokepoint",
		internalPrefix+"service/worktree",
		internalPrefix+"service/env",
		internalPrefix+"service/runjobs",
		internalPrefix+"flow/create",
		internalPrefix+"flow/run/seam",
		internalPrefix+"commands/wt",
		internalPrefix+"tui/dash",
	)
}

// A row that names no function can never fire: it reads as coverage and guards
// nothing, which is what happened to worktree.Reparent, Remove and Move.
func TestEveryMutatorIsADeclaredFunction(t *testing.T) {
	config := &packages.Config{Mode: packages.NeedName | packages.NeedTypes, Dir: "../.."}
	pkgs, err := packages.Load(config, "./internal/service/...")
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]*types.Package{}
	for _, pkg := range pkgs {
		byPath[pkg.PkgPath] = pkg.Types
	}
	for _, m := range mutators {
		pkg, ok := byPath[internalPrefix+m.pkg]
		if !ok {
			t.Errorf("%s: no such package", m.pkg)
			continue
		}
		if _, ok := pkg.Scope().Lookup(m.name).(*types.Func); !ok {
			t.Errorf("%s.%s is not a function of that package", m.pkg, m.name)
		}
	}
}
