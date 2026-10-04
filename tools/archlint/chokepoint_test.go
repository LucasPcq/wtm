package main

import (
	"go/constant"
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

// An event column naming no type the bus defines would make emits demand a
// publication nobody can write.
func TestEveryMutatorEventIsADeclaredEventType(t *testing.T) {
	config := &packages.Config{Mode: packages.NeedName | packages.NeedTypes, Dir: "../.."}
	pkgs, err := packages.Load(config, "./internal/domain")
	if err != nil || len(pkgs) != 1 {
		t.Fatalf("load domain: %v", err)
	}
	declared := eventTypesOf(pkgs[0].Types)
	if len(declared) == 0 {
		t.Fatal("domain declares no EventType constant")
	}
	for _, m := range mutators {
		for _, event := range m.events {
			if !declared[event] {
				t.Errorf("%s.%s publishes %q, which domain does not declare", m.pkg, m.name, event)
			}
		}
	}
}

// eventTypesOf reads the values of the domain's EventType constants.
func eventTypesOf(domain *types.Package) map[string]bool {
	declared := map[string]bool{}
	for _, name := range domain.Scope().Names() {
		c, ok := domain.Scope().Lookup(name).(*types.Const)
		if !ok || c.Val().Kind() != constant.String {
			continue
		}
		named, ok := c.Type().(*types.Named)
		if !ok || named.Obj().Name() != "EventType" {
			continue
		}
		declared[constant.StringVal(c.Val())] = true
	}
	return declared
}
