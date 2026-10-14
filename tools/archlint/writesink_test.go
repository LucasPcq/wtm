package main

import (
	"go/types"
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
	)
}

// The engine's services as docs/dev/services.md classifies them: every other
// exported function or method of the four packages is a reader.
var engineMutators = map[string]bool{
	"gitwt.Add":         true,
	"gitwt.Remove":      true,
	"identity.Allocate": true,
	"hooks.Execute":     true,
	"hooks.RunHooks":    true,
	"runenv.Apply":      true,
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

func exportedFuncs(pkg *types.Package) []*types.Func {
	var funcs []*types.Func
	for _, name := range pkg.Scope().Names() {
		switch obj := pkg.Scope().Lookup(name).(type) {
		case *types.Func:
			if obj.Exported() {
				funcs = append(funcs, obj)
			}
		case *types.TypeName:
			for method := range types.NewMethodSet(types.NewPointer(obj.Type())).Methods() {
				if fn, ok := method.Obj().(*types.Func); ok && fn.Exported() && fn.Pkg() == pkg {
					funcs = append(funcs, fn)
				}
			}
		}
	}
	return funcs
}
