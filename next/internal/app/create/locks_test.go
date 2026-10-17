package create_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm/operation"
	"github.com/stretchr/testify/assert"
)

func TestARunHoldsTheRepositoryAndEachBranchItCreates(t *testing.T) {
	locks := create.Request{Branches: []string{"feat/b", existing, ""}, From: "main"}.Locks()

	assert.Equal(t, []operation.Lock{
		{Scope: operation.ScopeRepo, Mode: operation.Shared},
		{Scope: operation.ScopeWorktree, Name: "feat/b", Mode: operation.Exclusive},
		{Scope: operation.ScopeWorktree, Name: existing, Mode: operation.Exclusive},
	}, locks)
}

func TestARunThatFastForwardsTheSourceHoldsItToo(t *testing.T) {
	w := newWorld(t, seed{Behind: true})
	facts := w.facts(t, branchName, "main")
	source := operation.Lock{Scope: operation.ScopeWorktree, Name: "main", Mode: operation.Exclusive}

	moved := create.Locks(facts, create.Request{Branches: []string{branchName}, From: "main", FastForward: new(true)})
	kept := create.Locks(facts, create.Request{Branches: []string{branchName}, From: "main", FastForward: new(false)})
	fromRemote := create.Locks(facts, create.Request{Branches: []string{branchName}, From: "origin/main", FastForward: new(true)})

	assert.Contains(t, moved, source)
	assert.NotContains(t, kept, source)
	assert.Equal(t, []operation.Lock{{Scope: operation.ScopeRepo, Mode: operation.Shared}, {Scope: operation.ScopeWorktree, Name: branchName, Mode: operation.Exclusive}}, fromRemote)
}

func TestALocalSourceNamedLikeARemoteOneIsLockedWhenItIsFastForwarded(t *testing.T) {
	w := newWorld(t, seed{})
	gittest.Git(t, w.main, "branch", "origin/foo", "main")
	ahead := gittest.Output(t, w.main, "commit-tree", "main^{tree}", "-p", "main", "-m", "upstream")
	gittest.Git(t, w.main, "update-ref", "refs/remotes/origin/origin/foo", ahead)
	req := create.Request{Branches: []string{branchName}, From: "origin/foo", FastForward: new(true)}
	facts := w.facts(t, branchName)

	planned, _, err := create.Plan(facts, req)
	require.NoError(t, err)

	assert.Equal(t, []string{"origin/foo"}, planned.FastForward)
	assert.Contains(t, create.Locks(facts, req), operation.Lock{Scope: operation.ScopeWorktree, Name: "origin/foo", Mode: operation.Exclusive})
}

func TestASourceThatWillNotMoveIsNotLocked(t *testing.T) {
	w := newWorld(t, seed{})
	req := create.Request{Branches: []string{branchName}, From: "main", FastForward: new(true)}

	assert.NotContains(t, create.Locks(w.facts(t, branchName), req), operation.Lock{Scope: operation.ScopeWorktree, Name: "main", Mode: operation.Exclusive})
}
