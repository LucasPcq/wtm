package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	domainstatefile "github.com/LucasPcq/wtm/internal/domain/statefile"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/rules/envcopy"
	"github.com/LucasPcq/wtm/internal/store"
	"github.com/LucasPcq/wtm/internal/testutil/storetest"
)

// The double a command is tested on behaves as the disk does: each case runs
// on the files and on storetest.Fake, from the same start (a record of feat/a,
// one written by a newer wtm, one that does not read, a config remembering an
// answer, a main checkout holding its .env) and must end the same.

type implementation struct {
	name       string
	meta       store.Meta
	config     store.ProjectConfig
	env        store.EnvFiles
	main       string
	remembered func() map[string]string
	file       func(path string) (string, bool)
}

var (
	featA = domain.WorktreeMetadata{SourceBranch: "main", CreatedAt: "2026-01-02T03:04:05Z", Ordinal: 2}
	newer = domain.WorktreeMetadata{Schema: 99, SourceBranch: "main"}
)

func implementations(t *testing.T) []implementation {
	t.Helper()
	return []implementation{onDisk(t), inMemory(t)}
}

func onDisk(t *testing.T) implementation {
	stateDir, main := t.TempDir(), t.TempDir()
	writeRecord(t, stateDir, "feat/a", featA)
	writeRecord(t, stateDir, "newer/b", newer)
	writeMeta(t, stateDir, "broken/d", "{not json")
	writeFile(t, filepath.Join(stateDir, domain.ConfigFileName), "[worktrees]\nbase_path = \"../.trees\"\n\n[wizard.remembered]\nisolation = \"verbatim\"\n")
	writeFile(t, filepath.Join(main, ".env"), userEnv)
	return implementation{
		name:   "disk",
		meta:   store.MetaDisk{StateDir: stateDir},
		config: store.ConfigDisk{StateDir: stateDir},
		env:    store.EnvDisk{},
		main:   main,
		remembered: func() map[string]string {
			project, err := config.LoadProjectRaw(stateDir)
			require.NoError(t, err)
			return project.Wizard.Remembered
		},
		file: func(path string) (string, bool) {
			data, err := os.ReadFile(path)
			return string(data), err == nil
		},
	}
}

func inMemory(t *testing.T) implementation {
	main := t.TempDir()
	fake := storetest.NewFake(storetest.FakeParams{
		Meta:       map[string]domain.WorktreeMetadata{"feat/a": featA, "newer/b": newer},
		Unreadable: []string{"broken/d"},
		Config:     &domain.ProjectConfig{Worktrees: domain.WorktreesConfig{BasePath: "../.trees"}, Wizard: domain.WizardConfig{Remembered: map[string]string{domain.RememberIsolation: "verbatim"}}},
		Files:      map[string]string{filepath.Join(main, ".env"): userEnv},
	})
	return implementation{
		name:   "fake",
		meta:   fake,
		config: fake,
		env:    fake,
		main:   main,
		remembered: func() map[string]string {
			project, ok := fake.Config()
			require.True(t, ok)
			return project.Wizard.Remembered
		},
		file: fake.File,
	}
}

func records(t *testing.T, meta store.Meta) []store.MetaRecord {
	t.Helper()
	all, err := meta.Records()
	require.NoError(t, err)
	return all
}

func TestTheFakeAndTheDiskAgreeOnTheRecords(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ctx := t.Context()
			setOrdinal := func(branch string, missing store.Missing) error {
				return errOf(impl.meta.Update(ctx, store.MetaUpdate{Branch: branch, Missing: missing, Change: func(m *domain.WorktreeMetadata) error {
					m.Ordinal = 5
					return nil
				}}))
			}

			require.NoError(t, setOrdinal("feat/a", store.MissingRefuse))
			require.NoError(t, setOrdinal("feat/c", store.MissingSkip))
			_, skipped := impl.meta.Load("feat/c")
			require.NoError(t, setOrdinal("feat/c", store.MissingCreate))
			created := records(t, impl.meta)
			require.NoError(t, impl.meta.Purge("feat/c"))
			_, purged := impl.meta.Load("feat/c")

			require.ErrorIs(t, skipped, os.ErrNotExist)
			assert.Equal(t, []store.MetaRecord{
				{Branch: "feat/a", Meta: domain.WorktreeMetadata{SourceBranch: "main", CreatedAt: "2026-01-02T03:04:05Z", Ordinal: 5}},
				{Branch: "feat/c", Meta: domain.WorktreeMetadata{Ordinal: 5}},
				{Branch: "newer/b", Meta: newer},
			}, created)
			require.ErrorIs(t, purged, os.ErrNotExist)
		})
	}
}

func TestTheFakeAndTheDiskSayWhatAnUpdateChanged(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ordinal := func(n int) func(*domain.WorktreeMetadata) error {
				return func(m *domain.WorktreeMetadata) error {
					m.Ordinal = n
					return nil
				}
			}

			changed, changedErr := impl.meta.Update(t.Context(), store.MetaUpdate{Branch: "feat/a", Change: ordinal(7)})
			same, sameErr := impl.meta.Update(t.Context(), store.MetaUpdate{Branch: "feat/a", Change: ordinal(7)})
			skipped, skippedErr := impl.meta.Update(t.Context(), store.MetaUpdate{Branch: "feat/z", Missing: store.MissingSkip, Change: ordinal(7)})

			require.NoError(t, changedErr)
			assert.Equal(t, store.Written{Changed: true}, changed)
			require.NoError(t, sameErr)
			assert.Equal(t, store.Written{}, same)
			require.NoError(t, skippedErr)
			assert.Equal(t, store.Written{}, skipped)

			kept := rules.RememberedChange{Remember: map[string]string{domain.RememberIsolation: "verbatim"}}
			_, err := impl.config.Remember(t.Context(), rules.RememberedChange{Remember: map[string]string{domain.RememberEnvStrategy: "parent"}})
			require.NoError(t, err)
			again, err := impl.config.Remember(t.Context(), kept)
			require.NoError(t, err)
			assert.Equal(t, store.Written{}, again)
		})
	}
}

func TestTheFakeAndTheDiskOverwriteARecordThatDoesNotReadOnlyWhenCreating(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			_, unreadable := impl.meta.Load("broken/d")

			written, err := impl.meta.Update(t.Context(), store.MetaUpdate{Branch: "broken/d", Missing: store.MissingCreate, Change: func(m *domain.WorktreeMetadata) error {
				m.Ordinal = 4
				return nil
			}})

			require.Error(t, unreadable)
			assert.NotErrorIs(t, unreadable, os.ErrNotExist)
			require.NoError(t, err)
			assert.True(t, written.Changed)
			meta, err := impl.meta.Load("broken/d")
			require.NoError(t, err)
			assert.Equal(t, 4, meta.Ordinal)
		})
	}
}

func TestTheFakeAndTheDiskAgreeOnTheMemory(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			require.NoError(t, errOf(impl.config.Remember(t.Context(), rules.RememberedChange{
				Remember: map[string]string{domain.RememberEnvStrategy: "parent"},
				Forget:   []string{domain.RememberIsolation},
			})))

			assert.Equal(t, map[string]string{domain.RememberEnvStrategy: "parent"}, impl.remembered())
		})
	}
}

func TestTheFakeAndTheDiskAgreeOnTheEnvFiles(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ctx := t.Context()
			target := t.TempDir()
			path := filepath.Join(target, ".env")

			copied, copyErr := impl.env.Copy(ctx, envcopy.Request{Strategy: domain.EnvStrategyMain, Files: []domain.EnvFile{{Target: ".env"}, {Target: ".env.local"}}, TargetDir: target, MainWorktreePath: impl.main})
			changed, editErr := impl.env.Edit(ctx, store.EnvEdit{Path: path, Edit: upsert("WEB_PORT", "3100")})
			unchanged, againErr := impl.env.Edit(ctx, store.EnvEdit{Path: path, Edit: upsert("WEB_PORT", "3100")})
			absent, absentErr := impl.env.Edit(ctx, store.EnvEdit{Path: filepath.Join(target, ".env.example"), Edit: upsert("WEB_PORT", "3100")})

			require.NoError(t, copyErr)
			assert.Equal(t, store.EnvCopied{Files: []string{path}, Warnings: []envcopy.Warning{envcopy.SourceMissing{Target: ".env.local", Dir: impl.main}}}, copied)
			require.NoError(t, editErr)
			assert.Equal(t, store.Written{Changed: true}, changed)
			require.NoError(t, againErr)
			assert.Equal(t, store.Written{}, unchanged)
			require.NoError(t, absentErr)
			assert.Equal(t, store.Written{}, absent)
			content, ok := impl.file(path)
			require.True(t, ok)
			assert.Equal(t, "# database\nDB_HOST=localhost\nWEB_PORT=3100 # the dev server\n\nexport API_KEY=secret\r\n", content)
			lines, readErr := impl.env.Read(path)
			require.NoError(t, readErr)
			assert.Equal(t, rules.ParseEnv(content), lines)
			_, exists := impl.file(filepath.Join(target, ".env.example"))
			assert.False(t, exists)
		})
	}
}

func TestTheFakeAndTheDiskRefuseTheSameThings(t *testing.T) {
	refusals := []struct {
		name string
		call func(t *testing.T, impl implementation) error
		is   error
	}{
		{name: "an absent record it must not create", is: os.ErrNotExist, call: func(t *testing.T, impl implementation) error {
			return errOf(impl.meta.Update(t.Context(), store.MetaUpdate{Branch: "feat/z", Missing: store.MissingRefuse, Change: func(*domain.WorktreeMetadata) error { return nil }}))
		}},
		{name: "a record a newer wtm wrote", is: domainstatefile.ErrSchemaNewer, call: func(t *testing.T, impl implementation) error {
			return errOf(impl.meta.Update(t.Context(), store.MetaUpdate{Branch: "newer/b", Change: func(m *domain.WorktreeMetadata) error {
				m.Ordinal = 1
				return nil
			}}))
		}},
		{name: "a change that fails", is: assert.AnError, call: func(t *testing.T, impl implementation) error {
			return errOf(impl.meta.Update(t.Context(), store.MetaUpdate{Branch: "feat/a", Change: func(m *domain.WorktreeMetadata) error {
				m.Ordinal = 9
				return assert.AnError
			}}))
		}},
		{name: "a record that does not read", call: func(t *testing.T, impl implementation) error {
			return errOf(impl.meta.Update(t.Context(), store.MetaUpdate{Branch: "broken/d", Change: func(m *domain.WorktreeMetadata) error {
				m.Ordinal = 1
				return nil
			}}))
		}},
		{name: "an unknown strategy", call: func(t *testing.T, impl implementation) error {
			_, err := impl.env.Copy(t.Context(), envcopy.Request{Strategy: "elsewhere", Files: []domain.EnvFile{{Target: ".env"}}, TargetDir: t.TempDir(), MainWorktreePath: impl.main})
			return err
		}},
	}
	for _, refusal := range refusals {
		for _, impl := range implementations(t) {
			t.Run(refusal.name+"/"+impl.name, func(t *testing.T) {
				before := records(t, impl.meta)

				err := refusal.call(t, impl)

				require.Error(t, err)
				if refusal.is != nil {
					require.ErrorIs(t, err, refusal.is)
				}
				assert.Equal(t, before, records(t, impl.meta))
				assert.Equal(t, map[string]string{domain.RememberIsolation: "verbatim"}, impl.remembered())
			})
		}
	}
}
