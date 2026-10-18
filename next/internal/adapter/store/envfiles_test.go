package store

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/env"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnOwnedKeyChangesNothingElseInTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	writeFile(t, path, userEnv)

	written, err := EnvFiles{}.Edit(t.Context(), dotenv.Edit{Path: path, Edit: upsert("WEB_PORT", "3100")})

	require.NoError(t, err)
	assert.Equal(t, domain.Written{Changed: true}, written)
	assert.Equal(t, "# database\nDB_HOST=localhost\nWEB_PORT=3100 # the dev server\n\nexport API_KEY=secret\r\n", readFile(t, path))
}

func TestANewOwnedKeyGoesAfterTheUsersLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	writeFile(t, path, userEnv)

	_, err := EnvFiles{}.Edit(t.Context(), dotenv.Edit{Path: path, Edit: upsert("COMPOSE_PROJECT_NAME", "app-feat-a")})

	require.NoError(t, err)
	assert.Equal(t, userEnv+"COMPOSE_PROJECT_NAME=app-feat-a\n", readFile(t, path))
}

func TestAnEditThatChangesNothingDoesNotRewrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	writeFile(t, path, userEnv)
	before, err := os.Stat(path)
	require.NoError(t, err)

	written, err := EnvFiles{}.Edit(t.Context(), dotenv.Edit{Path: path, Edit: upsert("WEB_PORT", "3000")})

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

	templateWritten, templateErr := EnvFiles{}.Edit(t.Context(), dotenv.Edit{Path: template, Edit: upsert("WEB_PORT", "3000")})
	valueWritten, valueErr := EnvFiles{}.Edit(t.Context(), dotenv.Edit{Path: value, Create: true, Edit: upsert("WEB_PORT", "3000")})

	require.NoError(t, templateErr)
	assert.False(t, templateWritten.Changed)
	assert.NoFileExists(t, template)
	require.NoError(t, valueErr)
	assert.True(t, valueWritten.Changed)
	assert.Equal(t, "WEB_PORT=3000", readFile(t, value))
}

func TestEditsFromSeveralGoroutinesAllLand(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	writeFile(t, path, userEnv)
	const edits = 12

	var wg sync.WaitGroup
	for i := range edits {
		wg.Go(func() {
			_, err := EnvFiles{}.Edit(t.Context(), dotenv.Edit{Path: path, Edit: upsert("KEY_"+strconv.Itoa(i), "1")})
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

	_, err := EnvFiles{}.Edit(t.Context(), dotenv.Edit{Path: path, Create: true, Edit: upsert("WEB_PORT", "3000")})

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
		strategy project.EnvStrategy
		file     project.EnvFile
		parent   string
		want     string
	}{
		{name: "example_pinned_template", strategy: project.EnvExample, file: project.EnvFile{Target: ".env", Template: ".env.dist"}, want: "FROM=dist"},
		{name: "example_first_candidate", strategy: project.EnvExample, file: project.EnvFile{Target: ".env"}, want: "FROM=example"},
		{name: "main", strategy: project.EnvMain, file: project.EnvFile{Target: ".env"}, want: "FROM=main"},
		{name: "main_nested", strategy: project.EnvMain, file: project.EnvFile{Target: "apps/api/.env"}, want: "FROM=main-api"},
		{name: "parent", strategy: project.EnvParent, file: project.EnvFile{Target: ".env"}, parent: parent, want: "FROM=parent"},
		{name: "parent_unknown", strategy: project.EnvParent, file: project.EnvFile{Target: ".env"}, want: "FROM=main"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := t.TempDir()

			copied, err := EnvFiles{}.Copy(t.Context(), env.CopyRequest{Strategy: tc.strategy, Files: []project.EnvFile{tc.file}, TargetDir: target, MainPath: main, ParentPath: tc.parent})

			require.NoError(t, err)
			assert.Empty(t, copied.Warnings)
			assert.Equal(t, []string{filepath.Join(target, tc.file.Target)}, copied.Files)
			assert.Equal(t, tc.want, readFile(t, filepath.Join(target, tc.file.Target)))
		})
	}
}

func TestCopyNamesWhatItCouldNotFind(t *testing.T) {
	copied, err := EnvFiles{}.Copy(t.Context(), env.CopyRequest{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: ".env.local"}}, TargetDir: t.TempDir(), MainPath: "/nowhere"})

	require.NoError(t, err)
	assert.Empty(t, copied.Files)
	assert.Equal(t, []env.CopyWarning{env.SourceMissing{Target: ".env.local", Dir: "/nowhere"}}, copied.Warnings)
}

func TestCopyRefusesAnUnknownStrategy(t *testing.T) {
	_, err := EnvFiles{}.Copy(t.Context(), env.CopyRequest{Strategy: "elsewhere", Files: []project.EnvFile{{Target: ".env"}}, TargetDir: t.TempDir(), MainPath: t.TempDir()})

	assert.EqualError(t, err, "unknown env strategy: elsewhere")
}

func TestCopyOfNoFileDoesNothing(t *testing.T) {
	target := t.TempDir()

	copied, err := EnvFiles{}.Copy(t.Context(), env.CopyRequest{Strategy: project.EnvExample, TargetDir: target, MainPath: t.TempDir()})

	require.NoError(t, err)
	assert.Equal(t, env.Copied{}, copied)
	entries, err := os.ReadDir(target)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestACopyOverAnIdenticalFileWritesNothing(t *testing.T) {
	main, target := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(main, ".env"), "FROM=main")
	writeFile(t, filepath.Join(target, ".env"), "FROM=main")

	copied, err := EnvFiles{}.Copy(t.Context(), env.CopyRequest{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: ".env"}}, TargetDir: target, MainPath: main})

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

	_, err := EnvFiles{}.Copy(t.Context(), env.CopyRequest{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: "apps/.env"}}, TargetDir: t.TempDir(), MainPath: main})

	require.ErrorIs(t, err, os.ErrPermission)
}

func TestReadTakesAnAbsentFileForNoLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")

	absent, err := EnvFiles{}.Read(path)
	require.NoError(t, err)
	assert.Nil(t, absent)

	writeFile(t, path, "A=1\n")
	lines, err := EnvFiles{}.Read(path)
	require.NoError(t, err)
	assert.Equal(t, "A=1\n", string(dotenv.Render(lines)))
}

func TestACopyKeepsTheModeOfItsSource(t *testing.T) {
	main, worktree := t.TempDir(), t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(main, ".env"), []byte("SECRET=1\n"), 0o600))

	_, err := EnvFiles{}.Copy(t.Context(), env.CopyRequest{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: ".env"}}, TargetDir: worktree, MainPath: main})
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(worktree, ".env"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestAnEnvFileAnEditCreatesIsReadableByItsOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")

	_, err := EnvFiles{}.Edit(t.Context(), dotenv.Edit{Path: path, Create: true, Edit: func(lines []dotenv.Line) []dotenv.Line {
		out, _ := dotenv.Set(dotenv.SetParams{Lines: lines, Key: "K", Value: "v"})
		return out
	}})
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
