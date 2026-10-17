package provision_test

import (
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWorktreeGetsTheSmallestOrdinalNoLiveWorktreeHolds(t *testing.T) {
	w, files := inMemory(t, seeds{
		Worktrees: []worktree.Worktree{
			{Path: "/trees/a", State: worktree.OnBranch{Branch: "feat/a"}},
			{Path: "/trees/b", State: worktree.Rebasing{Branch: "feat/b"}},
			{Path: "/trees/c", State: worktree.Detached{}},
			{Path: "/trees/d", State: worktree.OnBranch{Branch: "feat/d"}},
		},
		Store: storetest.FakeParams{Meta: map[string]worktree.Meta{"feat/a": {Ordinal: 1}, "feat/b": {Ordinal: 3}}},
	})

	outcome := provision.Run(t.Context(), portLinked(w.params(&recorder{})))

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, new(2), outcome.Provisioned.Ordinal)
	recorded, _ := files.Meta(branchName)
	assert.Equal(t, 2, recorded.Ordinal)
	assert.Equal(t, "main", recorded.SourceBranch)
}

func TestANumberIsGivenUnderTheOrdinalLock(t *testing.T) {
	w, files := inMemory(t, seeds{})

	provision.Run(t.Context(), portLinked(w.params(&recorder{})))

	calls := files.Calls()
	locked := slices.Index(calls, "LockOrdinals ")
	require.Positive(t, locked)
	assert.Equal(t, "Update "+branchName, calls[locked+1])
}

func TestANumberIsNeverGivenPastANeighbourWhoseRecordDoesNotRead(t *testing.T) {
	w, files := inMemory(t, seeds{
		Worktrees: []worktree.Worktree{{Path: "/trees/a", State: worktree.OnBranch{Branch: "feat/a"}}},
		Store:     storetest.FakeParams{Unreadable: []string{"feat/a"}},
	})

	outcome := provision.Run(t.Context(), portLinked(w.params(&recorder{})))

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Nil(t, outcome.Provisioned.Ordinal)
	recorded, _ := files.Meta(branchName)
	assert.Zero(t, recorded.Ordinal)
	require.Equal(t, []string{"provision.ports_not_settled"}, codes(outcome.Warnings))
	assert.Equal(t, "cannot read a live worktree's ordinal: feat/a: storetest: the record of feat/a does not read — ports not settled — run `wtm env feat/x` once this is fixed", outcome.Warnings[0].Message)
}

func TestANumberThatCannotBeGivenLeavesThePortsAsCopiedAndSaysWhy(t *testing.T) {
	w, _ := inMemory(t, seeds{GitFail: map[string]error{"Worktrees": errors.New("git worktree list: exit status 128")}})
	report := &recorder{}

	outcome := provision.Run(t.Context(), portLinked(w.params(report)))

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Nil(t, outcome.Provisioned.Ordinal)
	assert.Equal(t, []wtm.Warning{{
		Code:    "provision.ports_not_settled",
		Message: "list worktrees: git worktree list: exit status 128 — ports not settled — run `wtm env feat/x` once this is fixed",
		Params:  wtm.Params{"branch": branchName, "cause": "list worktrees: git worktree list: exit status 128"},
	}}, outcome.Warnings)
	assert.Len(t, report.worktreeEvents(), 2)
}

func TestANumberOnlyHooksNeededFailsSilently(t *testing.T) {
	w, _ := inMemory(t, seeds{GitFail: map[string]error{"Worktrees": errors.New("exit status 128")}})
	require.NoError(t, os.MkdirAll(w.path(branchName), 0o755))

	outcome := provision.Run(t.Context(), hooked(composed(w.params(&recorder{})), "true"))

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Nil(t, outcome.Provisioned.Ordinal)
	assert.Empty(t, outcome.Warnings)
}
