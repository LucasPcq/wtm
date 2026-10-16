package create_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/command/create"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestARunHoldsTheRepositoryAndEachBranchItCreates(t *testing.T) {
	w := newWorld(t, seed{})

	keys := create.Command(w.ports).Locks(create.Request{Branches: []string{branch, existing}, From: "main"})

	assert.Equal(t, []kernel.LockKey{
		{Scope: kernel.LockRepo, Mode: kernel.LockShared},
		{Scope: kernel.LockWorktree, Name: branch, Mode: kernel.LockExclusive},
		{Scope: kernel.LockWorktree, Name: existing, Mode: kernel.LockExclusive},
	}, keys)
}

func TestARunThatFastForwardsTheSourceHoldsItToo(t *testing.T) {
	w := newWorld(t, seed{})
	command := create.Command(w.ports)

	local := command.Locks(create.Request{Branches: []string{branch}, From: "main", FastForward: true})
	remote := command.Locks(create.Request{Branches: []string{branch}, From: "origin/main", FastForward: true})

	assert.Contains(t, local, kernel.LockKey{Scope: kernel.LockWorktree, Name: "main", Mode: kernel.LockExclusive})
	assert.NotContains(t, remote, kernel.LockKey{Scope: kernel.LockWorktree, Name: "origin/main", Mode: kernel.LockExclusive})
}

func TestTheCommandIsCreate(t *testing.T) {
	w := newWorld(t, seed{})

	assert.Equal(t, domain.CmdCreate, create.Command(w.ports).Name)
}
