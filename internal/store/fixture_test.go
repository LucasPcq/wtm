package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/store"
	"github.com/LucasPcq/wtm/internal/testutil/storetest"
)

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

const userEnv = "# database\nDB_HOST=localhost\nWEB_PORT=3000 # the dev server\n\nexport API_KEY=secret\r\n"

const sparseConfig = "[worktrees]\nbase_path = \"../.trees\"\n\n[[env.file]]\ntarget = \".env\"\n"
