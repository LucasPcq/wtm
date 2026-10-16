package create_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/command/create"
	"github.com/LucasPcq/wtm/internal/domain"
)

func TestAWorktreeIsAnnouncedCreatedNumberedThenProvisioned(t *testing.T) {
	w := newWorld(t, seed{Run: jobs})

	w.apply(t, create.Request{Branches: []string{branch}})

	assert.Equal(t, []domain.EventType{
		domain.EventOperationStarted,
		domain.EventWorktreeCreated,
		domain.EventWorktreeUpdated,
		domain.EventWorktreeProvisioned,
		domain.EventUnitFinished,
		domain.EventOperationFinished,
	}, w.bench.Bus.Types())
	created := w.bench.Bus.Events()[1].Worktree
	require.NotNil(t, created)
	assert.Equal(t, domain.WorktreeIdentity{Branch: branch, Path: w.path(branch), Parent: "main", Isolation: domain.IsolationIsolated, CreatedAt: "2026-10-16T12:00:00Z"}, *created)
	updated := w.bench.Bus.Events()[2]
	require.NotNil(t, updated.Worktree.Ordinal)
	assert.Equal(t, 1, *updated.Worktree.Ordinal)
	assert.Equal(t, []domain.IdentityField{domain.IdentityOrdinal}, updated.Changed)
}

func TestAHookThatFailsIsNamedInTheProvisionedEvent(t *testing.T) {
	w := newWorld(t, seed{Config: withHook("exit 3")})

	w.apply(t, create.Request{Branches: []string{branch}})

	provisioned := lastOf(w.bench.Bus.Events(), domain.EventWorktreeProvisioned)
	require.NotNil(t, provisioned.OK)
	assert.False(t, *provisioned.OK)
	assert.Equal(t, "exit 3", provisioned.Hook)
	require.NotNil(t, provisioned.ExitCode)
	assert.Equal(t, 3, *provisioned.ExitCode)
}
