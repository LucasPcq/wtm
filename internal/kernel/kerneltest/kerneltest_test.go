package kerneltest_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/kerneltest"
)

// spy stands in for *testing.T to see what a check reports.
type spy struct {
	testing.TB
	failures []string
}

func (s *spy) Helper() {}

func (s *spy) Errorf(format string, args ...any) {
	s.failures = append(s.failures, fmt.Sprintf(format, args...))
}

type options struct {
	Mode string `json:"mode"`
}

type request struct {
	Branches []string `json:"branches"`
	From     string   `json:"from"`
	Force    bool     `json:"force"`
	Options  options  `json:"options"`
}

type facts struct{ Base string }

func fromField(dependsOn ...string) kernel.FieldDef[request, facts] {
	return kernel.FieldDef[request, facts]{
		Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect, DependsOn: dependsOn},
		Skip: func(req request, _ facts) (bool, kernel.Code) { return req.Options.Mode == "detached", "test.detached" },
		Default: func(req request, f facts) (kernel.Fallback, bool) {
			return kernel.Fallback{Value: kernel.Value{Text: f.Base}, Origin: kernel.OriginDefault}, len(req.Branches) > 0
		},
	}
}

func TestCheckDependsOnPassesAFieldThatDeclaresWhatItReads(t *testing.T) {
	s := &spy{}
	kerneltest.CheckDependsOn(s, kerneltest.DependsOnParams[request, facts]{
		Fields:  []kernel.FieldDef[request, facts]{fromField("branches", "options")},
		Request: request{Branches: []string{"feat/x"}},
		Facts:   facts{Base: "main"},
	})
	if len(s.failures) != 0 {
		t.Errorf("failures = %v", s.failures)
	}
}

func TestCheckDependsOnCatchesEveryUndeclaredRead(t *testing.T) {
	s := &spy{}
	kerneltest.CheckDependsOn(s, kerneltest.DependsOnParams[request, facts]{
		Fields:  []kernel.FieldDef[request, facts]{fromField("branches")},
		Request: request{Branches: []string{"feat/x"}, Options: options{Mode: "detached"}},
		Facts:   facts{Base: "main"},
	})
	if len(s.failures) != 1 || !strings.Contains(s.failures[0], `"options.mode"`) {
		t.Errorf("failures = %v", s.failures)
	}
}

func TestCheckDependsOnRefusesAPathThatNamesNoField(t *testing.T) {
	s := &spy{}
	kerneltest.CheckDependsOn(s, kerneltest.DependsOnParams[request, facts]{
		Fields: []kernel.FieldDef[request, facts]{
			{Spec: kernel.FieldSpec{Path: "nope"}},
			{Spec: kernel.FieldSpec{Path: "force", DependsOn: []string{"ghost"}}},
		},
	})
	if len(s.failures) != 2 {
		t.Errorf("failures = %v", s.failures)
	}
}

type store struct{ entries []string }

func (s *store) add(name string) func(context.Context) error {
	return func(context.Context) error { s.entries = append(s.entries, name); return nil }
}

func (s *store) remove(name string) func(context.Context) error {
	return func(context.Context) error {
		s.entries = slices.DeleteFunc(s.entries, func(entry string) bool { return entry == name })
		return nil
	}
}

func (s *store) snapshot() string { return strings.Join(s.entries, ",") }

func TestCheckSagaPassesASagaThatUndoesEverything(t *testing.T) {
	disk := &store{entries: []string{"main"}}
	s := &spy{}
	kerneltest.CheckSaga(s, kerneltest.SagaParams[string]{
		Saga: func() kernel.Saga[string] {
			return kernel.Saga[string]{
				Steps: []kernel.SagaStep{
					{Name: "worktree", Do: disk.add("wt"), Undo: disk.remove("wt")},
					{Name: "meta", Do: disk.add("meta"), Undo: disk.remove("meta")},
				},
				Commit: func(context.Context) (string, error) { return "wt", nil },
			}
		},
		Snapshot: disk.snapshot,
	})
	if len(s.failures) != 0 {
		t.Errorf("failures = %v", s.failures)
	}
}

func TestCheckSagaCatchesAStepWithNothingToUndoIt(t *testing.T) {
	disk := &store{}
	s := &spy{}
	kerneltest.CheckSaga(s, kerneltest.SagaParams[string]{
		Saga: func() kernel.Saga[string] {
			return kernel.Saga[string]{Steps: []kernel.SagaStep{
				{Name: "worktree", Do: disk.add("wt"), Undo: disk.remove("wt")},
				{Name: "env", Do: disk.add(".env")},
				{Name: "meta", Do: disk.add("meta"), Undo: disk.remove("meta")},
			}}
		},
		Snapshot: disk.snapshot,
	})
	if len(s.failures) == 0 || !strings.Contains(strings.Join(s.failures, "\n"), "step meta") {
		t.Errorf("failures = %v", s.failures)
	}
}
