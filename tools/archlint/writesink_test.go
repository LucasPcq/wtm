package main

import (
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"
)

func TestAWriteIsReachedOnlyFromAMutatorsCaller(t *testing.T) {
	runTxtar(t, writesinkAnalyzer, "writesink",
		internalPrefix+"service/identity",
		internalPrefix+"service/gitwt",
		internalPrefix+"service/hooks",
		internalPrefix+"service/runenv",
		internalPrefix+"service/worktree",
		internalPrefix+"query/status",
		internalPrefix+"command/create",
		internalPrefix+"flow/create",
		internalPrefix+"surface/cli/wt",
		internalPrefix+"dispatch",
		internalPrefix+"registry",
	)
}

func TestTheEngineServicesAreClassifiedAsTheirDocSays(t *testing.T) {
	config := &packages.Config{Mode: loadMode, Dir: "../.."}
	pkgs, err := packages.Load(config, "./internal/service/gitwt", "./internal/service/identity", "./internal/service/runenv", "./internal/service/hooks")
	if err != nil {
		t.Fatal(err)
	}
	graph, err := checker.Analyze([]*analysis.Analyzer{writesinkAnalyzer}, pkgs, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, action := range graph.Roots {
		if action.Err != nil {
			t.Fatal(action.Err)
		}
		for _, fn := range exportedFuncs(action.Package.Types) {
			name := action.Package.Types.Name() + "." + fn.Name()
			found[name] = true
			if got := action.ObjectFact(fn, new(mutatorFact)); got != engineMutators[name] {
				t.Errorf("%s: mutator %v, want %v", name, got, engineMutators[name])
			}
		}
	}
	for name := range engineMutators {
		if !found[name] {
			t.Errorf("%s: no such function", name)
		}
	}
}
