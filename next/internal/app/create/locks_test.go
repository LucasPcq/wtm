package create_test

import (
	"testing"

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
	local := create.Request{Branches: []string{branchName}, From: "main", FastForward: new(true)}.Locks()
	remote := create.Request{Branches: []string{branchName}, From: "origin/main", FastForward: new(true)}.Locks()
	listed := create.Request{Branches: []string{branchName, "main"}, From: "main", FastForward: new(true)}.Locks()
	kept := create.Request{Branches: []string{branchName}, From: "main", FastForward: new(false)}.Locks()

	source := operation.Lock{Scope: operation.ScopeWorktree, Name: "main", Mode: operation.Exclusive}
	assert.Contains(t, local, source)
	assert.NotContains(t, remote, operation.Lock{Scope: operation.ScopeWorktree, Name: "origin/main", Mode: operation.Exclusive})
	assert.Len(t, listed, 3)
	assert.NotContains(t, kept, source)
}
