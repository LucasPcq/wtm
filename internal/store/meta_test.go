package store_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	domainstatefile "github.com/LucasPcq/wtm/internal/domain/statefile"
	"github.com/LucasPcq/wtm/internal/store"
)

func TestARecordWithoutSchemaReadsAsItAlwaysDid(t *testing.T) {
	stateDir := t.TempDir()
	writeMeta(t, stateDir, "feat/a", `{"source_branch": "main", "created_at": "2026-01-02T03:04:05Z", "env_strategy": "parent", "ordinal": 3, "namespaces": ["db"], "isolation": "verbatim"}`)

	meta, err := store.MetaDisk{StateDir: stateDir}.Load("feat/a")

	require.NoError(t, err)
	assert.Equal(t, domain.WorktreeMetadata{SourceBranch: "main", CreatedAt: "2026-01-02T03:04:05Z", EnvStrategy: domain.EnvStrategyParent, Ordinal: 3, Namespaces: []string{"db"}, Isolation: domain.IsolationVerbatim}, meta)
}

func TestARecordIsKeptUnderItsEncodedBranch(t *testing.T) {
	stateDir := t.TempDir()
	meta := store.MetaDisk{StateDir: stateDir}

	require.NoError(t, meta.Update(t.Context(), store.MetaUpdate{Branch: "feat/a", Missing: store.MissingCreate, Change: func(m *domain.WorktreeMetadata) error {
		m.SourceBranch = "main"
		return nil
	}}))

	assert.FileExists(t, filepath.Join(stateDir, domain.WorktreesSubdir, "feat%2Fa", domain.MetaFileName))
	assert.NotContains(t, readFile(t, metaPath(stateDir, "feat/a")), "schema", "the key is written from schema 2 on")
}

func TestLoadOfAnAbsentRecordIsNotExist(t *testing.T) {
	_, err := store.MetaDisk{StateDir: t.TempDir()}.Load("feat/a")

	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestUpdateChangesOnlyWhatItIsAskedTo(t *testing.T) {
	stateDir := t.TempDir()
	writeRecord(t, stateDir, "feat/a", domain.WorktreeMetadata{SourceBranch: "main", CreatedAt: "2026-01-02T03:04:05Z", Ordinal: 2})
	meta := store.MetaDisk{StateDir: stateDir}

	require.NoError(t, meta.Update(t.Context(), store.MetaUpdate{Branch: "feat/a", Change: func(m *domain.WorktreeMetadata) error {
		m.Isolation = domain.IsolationVerbatim
		return nil
	}}))

	got, err := meta.Load("feat/a")
	require.NoError(t, err)
	assert.Equal(t, domain.WorktreeMetadata{SourceBranch: "main", CreatedAt: "2026-01-02T03:04:05Z", Ordinal: 2, Isolation: domain.IsolationVerbatim}, got)
}

func TestUpdateOfAnAbsentRecordFollowsMissing(t *testing.T) {
	cases := map[store.Missing]struct {
		wantErr    bool
		wantRecord bool
	}{
		store.MissingCreate: {wantRecord: true},
		store.MissingSkip:   {},
		store.MissingRefuse: {wantErr: true},
	}
	for missing, want := range cases {
		t.Run(strconv.Itoa(int(missing)), func(t *testing.T) {
			stateDir := t.TempDir()
			meta := store.MetaDisk{StateDir: stateDir}

			err := meta.Update(t.Context(), store.MetaUpdate{Branch: "feat/a", Missing: missing, Change: func(m *domain.WorktreeMetadata) error {
				m.SourceBranch = "main"
				return nil
			}})

			if want.wantErr {
				require.ErrorIs(t, err, os.ErrNotExist)
			} else {
				require.NoError(t, err)
			}
			_, loadErr := meta.Load("feat/a")
			assert.Equal(t, want.wantRecord, loadErr == nil)
		})
	}
}

func TestAnUpdateThatSaysNothingOfAnAbsentRecordCreatesNone(t *testing.T) {
	meta := store.MetaDisk{StateDir: t.TempDir()}

	err := meta.Update(t.Context(), store.MetaUpdate{Branch: "feat/a", Change: func(m *domain.WorktreeMetadata) error {
		m.Ordinal = 1
		return nil
	}})

	require.ErrorIs(t, err, os.ErrNotExist)
	_, loadErr := meta.Load("feat/a")
	require.ErrorIs(t, loadErr, os.ErrNotExist)
}

func TestAnUnreadableRecordIsRebuiltOnlyWhenMissingCreates(t *testing.T) {
	stateDir := t.TempDir()
	writeMeta(t, stateDir, "feat/a", "{not json")
	meta := store.MetaDisk{StateDir: stateDir}
	setOrdinal := func(missing store.Missing) error {
		return meta.Update(t.Context(), store.MetaUpdate{Branch: "feat/a", Missing: missing, Change: func(m *domain.WorktreeMetadata) error {
			m.Ordinal = 4
			return nil
		}})
	}

	require.Error(t, setOrdinal(store.MissingRefuse))
	require.Error(t, setOrdinal(store.MissingSkip))
	assert.Equal(t, "{not json", readFile(t, metaPath(stateDir, "feat/a")))
	require.NoError(t, setOrdinal(store.MissingCreate))
	got, err := meta.Load("feat/a")
	require.NoError(t, err)
	assert.Equal(t, domain.WorktreeMetadata{Ordinal: 4}, got)
}

func TestARecordAtANewerSchemaIsReadButNeverRewritten(t *testing.T) {
	stateDir := t.TempDir()
	content := `{"schema": 99, "source_branch": "main"}`
	writeMeta(t, stateDir, "feat/a", content)
	meta := store.MetaDisk{StateDir: stateDir}

	got, loadErr := meta.Load("feat/a")
	err := meta.Update(t.Context(), store.MetaUpdate{Branch: "feat/a", Change: func(m *domain.WorktreeMetadata) error {
		m.Ordinal = 1
		return nil
	}})

	require.NoError(t, loadErr)
	assert.Equal(t, 99, got.Schema)
	require.ErrorIs(t, err, domainstatefile.ErrSchemaNewer)
	assert.Equal(t, content, readFile(t, metaPath(stateDir, "feat/a")))
}

func TestAChangeThatFailsWritesNothing(t *testing.T) {
	stateDir := t.TempDir()
	writeRecord(t, stateDir, "feat/a", domain.WorktreeMetadata{SourceBranch: "main"})
	before := readFile(t, metaPath(stateDir, "feat/a"))
	refused := assert.AnError

	err := store.MetaDisk{StateDir: stateDir}.Update(t.Context(), store.MetaUpdate{Branch: "feat/a", Change: func(m *domain.WorktreeMetadata) error {
		m.SourceBranch = "other"
		return refused
	}})

	require.ErrorIs(t, err, refused)
	assert.Equal(t, before, readFile(t, metaPath(stateDir, "feat/a")))
}

func TestMetaUpdatesFromSeveralGoroutinesAllLand(t *testing.T) {
	stateDir := t.TempDir()
	writeRecord(t, stateDir, "feat/a", domain.WorktreeMetadata{SourceBranch: "main"})
	meta := store.MetaDisk{StateDir: stateDir}
	const updates = 16

	var wg sync.WaitGroup
	for i := range updates {
		wg.Go(func() { assert.NoError(t, meta.Update(t.Context(), addNamespace("db"+strconv.Itoa(i)))) })
	}
	wg.Wait()

	got, err := meta.Load("feat/a")
	require.NoError(t, err)
	assert.Len(t, got.Namespaces, updates)
	assert.Equal(t, "main", got.SourceBranch)
}

func TestMetaUpdatesFromSeveralProcessesAllLand(t *testing.T) {
	stateDir := t.TempDir()
	writeRecord(t, stateDir, "feat/a", domain.WorktreeMetadata{SourceBranch: "main"})
	const processes, rounds = 4, 10

	var wg sync.WaitGroup
	for process := range processes {
		wg.Go(func() {
			cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
			cmd.Env = append(os.Environ(), envStateDir+"="+stateDir, envJob+"="+"p"+strconv.Itoa(process)+"-", envRounds+"="+strconv.Itoa(rounds))
			out, err := cmd.CombinedOutput()
			assert.NoError(t, err, string(out))
		})
	}
	wg.Wait()

	got, err := store.MetaDisk{StateDir: stateDir}.Load("feat/a")
	require.NoError(t, err)
	assert.Len(t, got.Namespaces, processes*rounds)
	assert.Equal(t, "main", got.SourceBranch)
}

func TestRecordsAreEveryRecordThatReadsByBranch(t *testing.T) {
	stateDir := t.TempDir()
	writeRecord(t, stateDir, "feat/b", domain.WorktreeMetadata{SourceBranch: "main"})
	writeRecord(t, stateDir, "feat/a", domain.WorktreeMetadata{Namespaces: []string{"db"}})
	writeMeta(t, stateDir, "broken", "{not json")
	require.NoError(t, os.MkdirAll(filepath.Join(stateDir, domain.WorktreesSubdir, "empty"), 0o755))

	records, err := store.MetaDisk{StateDir: stateDir}.Records()

	require.NoError(t, err)
	assert.Equal(t, []store.MetaRecord{
		{Branch: "feat/a", Meta: domain.WorktreeMetadata{Namespaces: []string{"db"}}},
		{Branch: "feat/b", Meta: domain.WorktreeMetadata{SourceBranch: "main"}},
	}, records)
}

func TestNoRecordsIsNoError(t *testing.T) {
	records, err := store.MetaDisk{StateDir: t.TempDir()}.Records()

	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestPurgeDropsTheRecordAndItsDirectory(t *testing.T) {
	stateDir := t.TempDir()
	writeRecord(t, stateDir, "feat/a", domain.WorktreeMetadata{Ordinal: 2})
	meta := store.MetaDisk{StateDir: stateDir}

	require.NoError(t, meta.Purge("feat/a"))

	assert.NoDirExists(t, filepath.Dir(metaPath(stateDir, "feat/a")))
	require.NoError(t, meta.Purge("feat/a"), "an absent record is already purged")
}

func TestPurgeNeverReachesOutsideItsRecord(t *testing.T) {
	stateDir := t.TempDir()
	writeRecord(t, stateDir, "feat/a", domain.WorktreeMetadata{Ordinal: 2})

	require.NoError(t, store.MetaDisk{StateDir: stateDir}.Purge(""))

	assert.FileExists(t, metaPath(stateDir, "feat/a"))
}
