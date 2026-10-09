package kerneltest_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/kerneltest"
)

func TestCheckDependsOnPassesAFieldThatDeclaresWhatItReads(t *testing.T) {
	s := &spy{}
	kerneltest.CheckDependsOn(s, kerneltest.DependsOnParams[request, facts]{
		Fields:  []kernel.FieldDef[request, facts]{fromField("branches", "options")},
		Request: request{Branches: []string{"feat/x"}},
		Facts:   facts{Base: "main"},
	})
	assert.Empty(t, s.failures)
}

func TestCheckDependsOnCatchesEveryUndeclaredRead(t *testing.T) {
	s := &spy{}
	kerneltest.CheckDependsOn(s, kerneltest.DependsOnParams[request, facts]{
		Fields:  []kernel.FieldDef[request, facts]{fromField("branches")},
		Request: request{Branches: []string{"feat/x"}, Options: options{Mode: "detached"}},
		Facts:   facts{Base: "main"},
	})
	require.Len(t, s.failures, 1)
	assert.Contains(t, s.failures[0], `"options.mode"`)
}

func TestCheckDependsOnRefusesAPathThatNamesNoField(t *testing.T) {
	s := &spy{}
	kerneltest.CheckDependsOn(s, kerneltest.DependsOnParams[request, facts]{
		Fields: []kernel.FieldDef[request, facts]{
			{Spec: kernel.FieldSpec{Path: "nope"}},
			{Spec: kernel.FieldSpec{Path: "force", DependsOn: []string{"ghost"}}},
		},
	})
	assert.Len(t, s.failures, 2)
}

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
	assert.Empty(t, s.failures)
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
	assert.Contains(t, strings.Join(s.failures, "\n"), "step meta")
}
