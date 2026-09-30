package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNextConfigReadsTheJobsOwnDirectory(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "next.config.ts"), []byte("export default {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, source := NextConfig(NextConfigParams{WorkDir: root, Cwd: "apps/web"})
	if source != "export default {}\n" {
		t.Errorf("source = %q, want the file's content", source)
	}
	if filepath.Base(path) != "next.config.ts" {
		t.Errorf("path = %q, want the file the user has to edit", path)
	}
}

func TestNextConfigIsEmptyWithoutOne(t *testing.T) {
	path, source := NextConfig(NextConfigParams{WorkDir: t.TempDir()})
	if path != "" || source != "" {
		t.Errorf("got %q, %q — a project with no next.config has nothing to say", path, source)
	}
}
