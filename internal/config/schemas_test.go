package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/schemas"
)

func staleSchema(t *testing.T, dir string, schema schemas.Schema) string {
	t.Helper()
	path := filepath.Join(dir, domain.SchemasDirName, schema.Filename())
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"stale": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertSchema(t *testing.T, path string, schema schemas.Schema) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !bytes.Equal(got, schema.Bytes()) {
		t.Errorf("%s was not refreshed to this binary's schema", path)
	}
}

// A config file points its editor at ./schemas/<file>.json: whoever rewrites
// the file rewrites the schema beside it, or an upgrade leaves the editor
// flagging every key the new version added.
func TestWritingAConfigRefreshesItsSchema(t *testing.T) {
	t.Run("run.toml", func(t *testing.T) {
		dir := t.TempDir()
		path := staleSchema(t, dir, schemas.Run)
		if err := WriteRun(WriteRunParams{StateDir: dir, Force: true}); err != nil {
			t.Fatal(err)
		}
		assertSchema(t, path, schemas.Run)
	})
	t.Run("config.toml", func(t *testing.T) {
		dir := t.TempDir()
		path := staleSchema(t, dir, schemas.Project)
		if err := WriteProjectConfig(WriteProjectConfigParams{StateDir: dir, Config: domain.ProjectConfig{Worktrees: domain.WorktreesConfig{BasePath: "../t", BaseBranch: "main"}}}); err != nil {
			t.Fatal(err)
		}
		assertSchema(t, path, schemas.Project)
	})
	t.Run("global config.toml", func(t *testing.T) {
		dir := t.TempDir()
		path := staleSchema(t, dir, schemas.Global)
		if err := writeGlobalAt(filepath.Join(dir, domain.GlobalConfigFile), domain.InitGlobalAnswers{Shell: domain.ShellZsh}); err != nil {
			t.Fatal(err)
		}
		assertSchema(t, path, schemas.Global)
	})
}
