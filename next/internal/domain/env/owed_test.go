package env

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestADropOwedKeepsItsNameAndTheVariablesItRunsWith(t *testing.T) {
	db := project.Job{Name: "db", Scope: project.Shared, Namespace: &project.Namespace{Name: "acme_{worktree}", Create: "createdb", Remove: "dropdb"}}

	drop, owed, err := NamespaceDrop(db, map[string]string{WorktreeKey: "feat-a", ordinalKey: "2", "PGPORT": "5433"})
	debt := Owe("db", drop)
	again, droppable, againErr := debt.Drop(db)

	require.NoError(t, err)
	require.NoError(t, againErr)
	assert.True(t, owed)
	assert.Equal(t, OwedDrop{Job: "db", Worktree: "feat-a", Ordinal: 2, Name: "acme_feat-a", Env: drop.Env}, debt)
	assert.Equal(t, "5433", drop.Env["PGPORT"])
	assert.True(t, droppable)
	assert.Equal(t, drop, again)
}

func TestADropTheLegacyWtmOwedIsMadeFromItsSlugAndOrdinal(t *testing.T) {
	db := project.Job{Name: "db", Scope: project.Shared, Namespace: &project.Namespace{Name: "acme_{worktree}_{ordinal}", Create: "createdb", Remove: "dropdb"}}

	drop, droppable, err := OwedDrop{Job: "db", Worktree: "feat-a", Ordinal: 2}.Drop(db)

	require.NoError(t, err)
	assert.True(t, droppable)
	assert.Equal(t, "acme_feat-a_2", drop.Name)
	assert.Equal(t, "acme_feat-a_2", drop.Env[NamespaceKey])
}

func TestANamespaceWithNoRemoveCommandIsNothingToDrop(t *testing.T) {
	db := project.Job{Name: "db", Scope: project.Shared, Namespace: &project.Namespace{Name: "acme_{worktree}", Create: "createdb"}}

	_, owed, err := NamespaceDrop(db, map[string]string{WorktreeKey: "feat-a"})

	require.NoError(t, err)
	assert.False(t, owed)
}
