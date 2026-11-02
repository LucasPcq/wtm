package store

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAQueueTheLegacyWtmWroteReadsWithItsKeys(t *testing.T) {
	stateDir := t.TempDir()
	writeFile(t, filepath.Join(stateDir, "pending-removals.toml"), legacyOwed)

	owed, err := Owed{StateDir: stateDir}.Load()

	require.NoError(t, err)
	assert.Equal(t, []env.OwedDrop{{Job: "db", Worktree: "feat-a", Ordinal: 2}}, owed)
}

func TestAQueueKeepingANameIsWrittenUnderSchemaTwo(t *testing.T) {
	stateDir := t.TempDir()
	owed := Owed{StateDir: stateDir}
	drop := env.OwedDrop{Job: "db", Worktree: "feat-b", Ordinal: 3, Name: "acme_feat_b", Env: map[string]string{"PGPORT": "5433"}}

	require.NoError(t, owed.Update(t.Context(), func(held []env.OwedDrop) []env.OwedDrop { return append(held, drop) }))

	assert.Equal(t, "schema = 2\n\n[[pending]]\n  job = \"db\"\n  worktree = \"feat-b\"\n  ordinal = 3\n  name = \"acme_feat_b\"\n  [pending.env]\n    PGPORT = \"5433\"\n", readFile(t, filepath.Join(stateDir, "pending-removals.toml")))
	loaded, err := owed.Load()
	require.NoError(t, err)
	assert.Equal(t, []env.OwedDrop{drop}, loaded)
}

func TestALegacyEntryAloneIsWrittenWithoutSchema(t *testing.T) {
	stateDir := t.TempDir()
	writeFile(t, filepath.Join(stateDir, "pending-removals.toml"), legacyOwed)

	require.NoError(t, Owed{StateDir: stateDir}.Update(t.Context(), func(held []env.OwedDrop) []env.OwedDrop { return held }))

	assert.Equal(t, "[[pending]]\n  job = \"db\"\n  worktree = \"feat-a\"\n  ordinal = 2\n", readFile(t, filepath.Join(stateDir, "pending-removals.toml")))
}

func TestTheQueueGoesOnceNothingIsOwed(t *testing.T) {
	stateDir := t.TempDir()
	writeFile(t, filepath.Join(stateDir, "pending-removals.toml"), legacyOwed)

	require.NoError(t, Owed{StateDir: stateDir}.Update(t.Context(), func([]env.OwedDrop) []env.OwedDrop { return nil }))

	assert.NoFileExists(t, filepath.Join(stateDir, "pending-removals.toml"))
}

func TestAQueueANewerWtmWroteIsRefused(t *testing.T) {
	stateDir := t.TempDir()
	writeFile(t, filepath.Join(stateDir, "pending-removals.toml"), "schema = 3\n"+legacyOwed)
	owed := Owed{StateDir: stateDir}

	_, loadErr := owed.Load()
	updateErr := owed.Update(t.Context(), func(held []env.OwedDrop) []env.OwedDrop { return nil })

	var newer *domain.SchemaNewer
	assert.True(t, errors.As(loadErr, &newer))
	assert.True(t, errors.As(updateErr, &newer))
	assert.FileExists(t, filepath.Join(stateDir, "pending-removals.toml"))
}
