package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARecordWithoutSchemaReadsAsItAlwaysDid(t *testing.T) {
	stateDir := t.TempDir()
	writeMeta(t, stateDir, "feat/a", `{"source_branch": "main", "created_at": "2026-01-02T03:04:05Z", "env_strategy": "parent", "ordinal": 3, "namespaces": ["db"], "isolation": "verbatim"}`)

	meta, err := Meta{StateDir: stateDir}.Load("feat/a")

	require.NoError(t, err)
	assert.Equal(t, worktree.Meta{SourceBranch: "main", CreatedAt: "2026-01-02T03:04:05Z", EnvStrategy: project.EnvParent, Ordinal: 3, Namespaces: []string{"db"}, Isolation: project.Verbatim}, meta)
}

func TestARecordIsWrittenByteForByteAsWtmWritesIt(t *testing.T) {
	stateDir := t.TempDir()
	meta := Meta{StateDir: stateDir}

	written, err := meta.Update(t.Context(), worktree.MetaUpdate{Branch: "refactor/next-create-pilot", Change: func(m *worktree.Meta) error {
		m.SourceBranch = "0.30.0"
		m.CreatedAt = "2026-10-16T20:37:05Z"
		m.EnvStrategy = project.EnvParent
		m.Isolation = project.Isolated
		return nil
	}})

	require.NoError(t, err)
	assert.True(t, written.Changed)
	path := filepath.Join(stateDir, "worktrees", "refactor%2Fnext-create-pilot", "meta.json")
	assert.Equal(t, realRecord, readFile(t, path))
	loaded, err := meta.Load("refactor/next-create-pilot")
	require.NoError(t, err)
	assert.Equal(t, worktree.Meta{SourceBranch: "0.30.0", CreatedAt: "2026-10-16T20:37:05Z", EnvStrategy: project.EnvParent, Isolation: project.Isolated}, loaded)
}

func TestARecordWtmWroteReadsAndRewritesToTheSameBytes(t *testing.T) {
	stateDir := t.TempDir()
	writeMeta(t, stateDir, "refactor/next-create-pilot", realRecord)
	meta := Meta{StateDir: stateDir}

	written, err := meta.Update(t.Context(), worktree.MetaUpdate{Branch: "refactor/next-create-pilot", Change: func(*worktree.Meta) error { return nil }})

	require.NoError(t, err)
	assert.False(t, written.Changed)
	assert.Equal(t, realRecord, readFile(t, metaPath(t, stateDir, "refactor/next-create-pilot")))
}

func TestARecordIsKeptUnderItsEncodedBranch(t *testing.T) {
	stateDir := t.TempDir()
	meta := Meta{StateDir: stateDir}

	require.NoError(t, errOf(meta.Update(t.Context(), worktree.MetaUpdate{Branch: "feat/a", Change: func(m *worktree.Meta) error {
		m.SourceBranch = "main"
		return nil
	}})))

	assert.FileExists(t, filepath.Join(stateDir, "worktrees", "feat%2Fa", "meta.json"))
	assert.NotContains(t, readFile(t, metaPath(t, stateDir, "feat/a")), "schema", "the key is written from schema 2 on")
}

func TestLoadOfAnAbsentRecordIsNotExist(t *testing.T) {
	_, err := Meta{StateDir: t.TempDir()}.Load("feat/a")

	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestABranchThatIsNoPathSegmentHasNoRecord(t *testing.T) {
	meta := Meta{StateDir: t.TempDir()}

	_, loadErr := meta.Load("..")
	_, updateErr := meta.Update(t.Context(), worktree.MetaUpdate{Branch: "..", Change: func(*worktree.Meta) error { return nil }})

	assert.Error(t, loadErr)
	assert.Error(t, updateErr)
}

func TestUpdateChangesOnlyWhatItIsAskedTo(t *testing.T) {
	stateDir := t.TempDir()
	writeRecord(t, stateDir, "feat/a", worktree.Meta{SourceBranch: "main", CreatedAt: "2026-01-02T03:04:05Z", Ordinal: 2})
	meta := Meta{StateDir: stateDir}

	require.NoError(t, errOf(meta.Update(t.Context(), worktree.MetaUpdate{Branch: "feat/a", Change: func(m *worktree.Meta) error {
		m.Isolation = project.Verbatim
		return nil
	}})))

	got, err := meta.Load("feat/a")
	require.NoError(t, err)
	assert.Equal(t, worktree.Meta{SourceBranch: "main", CreatedAt: "2026-01-02T03:04:05Z", Ordinal: 2, Isolation: project.Verbatim}, got)
}

func TestUpdateOfAnAbsentRecordCreatesIt(t *testing.T) {
	meta := Meta{StateDir: t.TempDir()}

	_, err := meta.Update(t.Context(), worktree.MetaUpdate{Branch: "feat/a", Change: func(m *worktree.Meta) error {
		m.SourceBranch = "main"
		return nil
	}})

	require.NoError(t, err)
	got, err := meta.Load("feat/a")
	require.NoError(t, err)
	assert.Equal(t, "main", got.SourceBranch)
}

func TestAnUnreadableRecordIsRebuilt(t *testing.T) {
	stateDir := t.TempDir()
	writeMeta(t, stateDir, "feat/a", "{not json")
	meta := Meta{StateDir: stateDir}

	_, err := meta.Update(t.Context(), worktree.MetaUpdate{Branch: "feat/a", Change: func(m *worktree.Meta) error {
		m.Ordinal = 4
		return nil
	}})

	require.NoError(t, err)
	got, err := meta.Load("feat/a")
	require.NoError(t, err)
	assert.Equal(t, worktree.Meta{Ordinal: 4}, got)
}

func TestARecordAtANewerSchemaIsReadButNeverRewritten(t *testing.T) {
	stateDir := t.TempDir()
	content := `{"schema": 99, "source_branch": "main"}`
	writeMeta(t, stateDir, "feat/a", content)
	meta := Meta{StateDir: stateDir}

	got, loadErr := meta.Load("feat/a")
	_, err := meta.Update(t.Context(), worktree.MetaUpdate{Branch: "feat/a", Change: func(m *worktree.Meta) error {
		m.Ordinal = 1
		return nil
	}})

	require.NoError(t, loadErr)
	assert.Equal(t, 99, got.Schema)
	var newer *domain.SchemaNewer
	require.ErrorAs(t, err, &newer)
	assert.Equal(t, 99, newer.Found)
	assert.Equal(t, 1, newer.Supported)
	assert.Equal(t, content, readFile(t, metaPath(t, stateDir, "feat/a")))
}

func TestAChangeThatFailsWritesNothing(t *testing.T) {
	stateDir := t.TempDir()
	writeRecord(t, stateDir, "feat/a", worktree.Meta{SourceBranch: "main"})
	before := readFile(t, metaPath(t, stateDir, "feat/a"))
	refused := assert.AnError

	_, err := Meta{StateDir: stateDir}.Update(t.Context(), worktree.MetaUpdate{Branch: "feat/a", Change: func(m *worktree.Meta) error {
		m.SourceBranch = "other"
		return refused
	}})

	require.ErrorIs(t, err, refused)
	assert.Equal(t, before, readFile(t, metaPath(t, stateDir, "feat/a")))
}

func TestMetaUpdatesFromSeveralGoroutinesAllLand(t *testing.T) {
	stateDir := t.TempDir()
	writeRecord(t, stateDir, "feat/a", worktree.Meta{SourceBranch: "main"})
	meta := Meta{StateDir: stateDir}
	const updates = 16

	var wg sync.WaitGroup
	for i := range updates {
		wg.Go(func() { assert.NoError(t, errOf(meta.Update(t.Context(), addNamespace("db"+strconv.Itoa(i))))) })
	}
	wg.Wait()

	got, err := meta.Load("feat/a")
	require.NoError(t, err)
	assert.Len(t, got.Namespaces, updates)
	assert.Equal(t, "main", got.SourceBranch)
}

func TestMetaUpdatesFromSeveralProcessesAllLand(t *testing.T) {
	stateDir := t.TempDir()
	writeRecord(t, stateDir, "feat/a", worktree.Meta{SourceBranch: "main"})
	const processes, rounds = 4, 10

	var wg sync.WaitGroup
	for process := range processes {
		wg.Go(func() {
			cmd := exec.Command(os.Args[0], "-test.run=^TestTheProcessTheCrossProcessTestStarts$")
			cmd.Env = append(os.Environ(), envStateDir+"="+stateDir, envJob+"="+"p"+strconv.Itoa(process)+"-", envRounds+"="+strconv.Itoa(rounds))
			out, err := cmd.CombinedOutput()
			assert.NoError(t, err, string(out))
		})
	}
	wg.Wait()

	got, err := Meta{StateDir: stateDir}.Load("feat/a")
	require.NoError(t, err)
	assert.Len(t, got.Namespaces, processes*rounds)
	assert.Equal(t, "main", got.SourceBranch)
}

func TestTheProcessTheCrossProcessTestStarts(t *testing.T) {
	stateDir := os.Getenv(envStateDir)
	if stateDir == "" {
		t.Skip("run by TestMetaUpdatesFromSeveralProcessesAllLand")
	}
	rounds, err := strconv.Atoi(os.Getenv(envRounds))
	require.NoError(t, err)
	meta := Meta{StateDir: stateDir}
	for round := range rounds {
		require.NoError(t, errOf(meta.Update(t.Context(), addNamespace(os.Getenv(envJob)+strconv.Itoa(round)))))
	}
}

func TestNumberingRunsUnderTheOrdinalLockWtmTakes(t *testing.T) {
	stateDir := t.TempDir()
	ran := false

	held, err := Meta{StateDir: stateDir}.LockOrdinals(t.Context(), func() error { ran = true; return nil })

	require.NoError(t, err)
	assert.True(t, held)
	assert.True(t, ran)
	assert.FileExists(t, filepath.Join(stateDir, "ordinal.lock"))
}

func TestNumberingWhereTheLockCannotBeTakenRunsUnlocked(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(stateDir, nil, 0o644))
	ran := false

	held, err := Meta{StateDir: stateDir}.LockOrdinals(t.Context(), func() error { ran = true; return nil })

	require.NoError(t, err)
	assert.False(t, held)
	assert.True(t, ran)
}

func TestTheRecordsListedAreTheBranchesTheyBelongTo(t *testing.T) {
	stateDir := t.TempDir()
	meta := Meta{StateDir: stateDir}
	empty, err := meta.List()
	require.NoError(t, err)
	writeMeta(t, stateDir, "feat/a", realRecord)
	writeMeta(t, stateDir, "main", realRecord)

	listed, err := meta.List()

	require.NoError(t, err)
	assert.Empty(t, empty)
	assert.ElementsMatch(t, []string{"feat/a", "main"}, listed)
}
