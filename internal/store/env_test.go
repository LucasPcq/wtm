package store_test

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/rules/envcopy"
	"github.com/LucasPcq/wtm/internal/store"
)

const userEnv = "# database\nDB_HOST=localhost\nWEB_PORT=3000 # the dev server\n\nexport API_KEY=secret\r\n"

func upsert(key, value string) func([]domain.EnvLine) []domain.EnvLine {
	return func(lines []domain.EnvLine) []domain.EnvLine {
		updated, _ := rules.UpsertEnvPair(rules.UpsertEnvPairParams{Lines: lines, Key: key, Value: value})
		return updated
	}
}

func TestAnOwnedKeyChangesNothingElseInTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	writeFile(t, path, userEnv)

	written, err := store.EnvDisk{}.Edit(t.Context(), store.EnvEdit{Path: path, Edit: upsert("WEB_PORT", "3100")})

	require.NoError(t, err)
	assert.Equal(t, store.Written{Changed: true}, written)
	assert.Equal(t, "# database\nDB_HOST=localhost\nWEB_PORT=3100 # the dev server\n\nexport API_KEY=secret\r\n", readFile(t, path))
}

func TestANewOwnedKeyGoesAfterTheUsersLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	writeFile(t, path, userEnv)

	_, err := store.EnvDisk{}.Edit(t.Context(), store.EnvEdit{Path: path, Edit: upsert("COMPOSE_PROJECT_NAME", "app-feat-a")})

	require.NoError(t, err)
	assert.Equal(t, userEnv+"COMPOSE_PROJECT_NAME=app-feat-a\n", readFile(t, path))
}

func TestAnEditThatChangesNothingDoesNotRewrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	writeFile(t, path, userEnv)
	before, err := os.Stat(path)
	require.NoError(t, err)

	written, err := store.EnvDisk{}.Edit(t.Context(), store.EnvEdit{Path: path, Edit: upsert("WEB_PORT", "3000")})

	require.NoError(t, err)
	assert.False(t, written.Changed)
	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime())
}

func TestAnAbsentFileIsCreatedOnlyWhenAsked(t *testing.T) {
	dir := t.TempDir()
	template := filepath.Join(dir, ".env.example")
	value := filepath.Join(dir, ".env")

	templateChanged, templateErr := store.EnvDisk{}.Edit(t.Context(), store.EnvEdit{Path: template, Edit: upsert("WEB_PORT", "3000")})
	valueChanged, valueErr := store.EnvDisk{}.Edit(t.Context(), store.EnvEdit{Path: value, Create: true, Edit: upsert("WEB_PORT", "3000")})

	require.NoError(t, templateErr)
	assert.False(t, templateChanged.Changed)
	assert.NoFileExists(t, template)
	require.NoError(t, valueErr)
	assert.True(t, valueChanged.Changed)
	assert.Equal(t, "WEB_PORT=3000", readFile(t, value))
}

func TestEditsFromSeveralGoroutinesAllLand(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	writeFile(t, path, userEnv)
	const edits = 12

	var wg sync.WaitGroup
	for i := range edits {
		wg.Go(func() {
			_, err := store.EnvDisk{}.Edit(t.Context(), store.EnvEdit{Path: path, Edit: upsert("KEY_"+strconv.Itoa(i), "1")})
			assert.NoError(t, err)
		})
	}
	wg.Wait()

	content := readFile(t, path)
	for i := range edits {
		assert.Contains(t, content, "KEY_"+strconv.Itoa(i)+"=1\n")
	}
	assert.Contains(t, content, userEnv)
}

func TestNoLockFileIsLeftInTheUsersTree(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	_, err := store.EnvDisk{}.Edit(t.Context(), store.EnvEdit{Path: path, Create: true, Edit: upsert("WEB_PORT", "3000")})

	require.NoError(t, err)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, ".env", entries[0].Name())
}

func TestCopyTakesEachFileFromItsStrategysSource(t *testing.T) {
	main, parent := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(main, ".env.example"), "FROM=example")
	writeFile(t, filepath.Join(main, ".env.dist"), "FROM=dist")
	writeFile(t, filepath.Join(main, ".env.sample"), "FROM=sample")
	writeFile(t, filepath.Join(main, ".env"), "FROM=main")
	writeFile(t, filepath.Join(main, "apps", "api", ".env"), "FROM=main-api")
	writeFile(t, filepath.Join(parent, ".env"), "FROM=parent")
	cases := []struct {
		name     string
		strategy domain.EnvStrategy
		file     domain.EnvFile
		parent   string
		want     string
	}{
		{name: "example, pinned template", strategy: domain.EnvStrategyExample, file: domain.EnvFile{Target: ".env", Template: ".env.dist"}, want: "FROM=dist"},
		{name: "example, first candidate", strategy: domain.EnvStrategyExample, file: domain.EnvFile{Target: ".env"}, want: "FROM=example"},
		{name: "main", strategy: domain.EnvStrategyMain, file: domain.EnvFile{Target: ".env"}, want: "FROM=main"},
		{name: "main, nested", strategy: domain.EnvStrategyMain, file: domain.EnvFile{Target: "apps/api/.env"}, want: "FROM=main-api"},
		{name: "parent", strategy: domain.EnvStrategyParent, file: domain.EnvFile{Target: ".env"}, parent: parent, want: "FROM=parent"},
		{name: "parent unknown", strategy: domain.EnvStrategyParent, file: domain.EnvFile{Target: ".env"}, want: "FROM=main"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			target := t.TempDir()

			copied, err := store.EnvDisk{}.Copy(t.Context(), envcopy.Request{Strategy: c.strategy, Files: []domain.EnvFile{c.file}, TargetDir: target, MainWorktreePath: main, ParentWorktreePath: c.parent})

			require.NoError(t, err)
			assert.Empty(t, copied.Warnings)
			assert.Equal(t, []string{filepath.Join(target, c.file.Target)}, copied.Files)
			assert.Equal(t, c.want, readFile(t, filepath.Join(target, c.file.Target)))
		})
	}
}

func TestCopyRefusesAnUnknownStrategy(t *testing.T) {
	_, err := store.EnvDisk{}.Copy(t.Context(), envcopy.Request{Strategy: "elsewhere", Files: []domain.EnvFile{{Target: ".env"}}, TargetDir: t.TempDir(), MainWorktreePath: t.TempDir()})

	require.Error(t, err)
}

func TestCopyOfNoFileDoesNothing(t *testing.T) {
	target := t.TempDir()

	copied, err := store.EnvDisk{}.Copy(t.Context(), envcopy.Request{Strategy: domain.EnvStrategyExample, TargetDir: target, MainWorktreePath: t.TempDir()})

	require.NoError(t, err)
	assert.Equal(t, store.EnvCopied{}, copied)
	entries, err := os.ReadDir(target)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestACopyOverAnIdenticalFileWritesNothing(t *testing.T) {
	main, target := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(main, ".env"), "FROM=main")
	writeFile(t, filepath.Join(target, ".env"), "FROM=main")

	copied, err := store.EnvDisk{}.Copy(t.Context(), envcopy.Request{Strategy: domain.EnvStrategyMain, Files: []domain.EnvFile{{Target: ".env"}}, TargetDir: target, MainWorktreePath: main})

	require.NoError(t, err)
	assert.Empty(t, copied.Files)
}

func TestACopySourceThatCannotBeStatedIsAnErrorNotASkip(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads through any mode")
	}
	main := t.TempDir()
	sealed := filepath.Join(main, "apps")
	writeFile(t, filepath.Join(sealed, ".env"), "FROM=main")
	require.NoError(t, os.Chmod(sealed, 0o000))
	t.Cleanup(func() { _ = os.Chmod(sealed, 0o755) })

	_, err := store.EnvDisk{}.Copy(t.Context(), envcopy.Request{Strategy: domain.EnvStrategyMain, Files: []domain.EnvFile{{Target: "apps/.env"}}, TargetDir: t.TempDir(), MainWorktreePath: main})

	require.ErrorIs(t, err, os.ErrPermission)
}

func TestReadTakesAnAbsentFileForNoLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")

	absent, err := store.EnvDisk{}.Read(path)
	require.NoError(t, err)
	assert.Nil(t, absent)

	writeFile(t, path, "A=1\n")
	lines, err := store.EnvDisk{}.Read(path)
	require.NoError(t, err)
	assert.Equal(t, "A=1\n", rules.RenderEnv(lines))
}

func TestTheTemplateIsThePinnedOneElseTheFirstCandidateOnDisk(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env.sample"), "")

	found, err := store.EnvDisk{}.Template(dir, domain.EnvFile{Target: ".env"})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, ".env.sample"), found)
	pinned, err := store.EnvDisk{}.Template(dir, domain.EnvFile{Target: ".env", Template: ".env.example"})
	require.NoError(t, err)
	assert.Empty(t, pinned, "a pinned template that is gone is not replaced by a guess")
}
