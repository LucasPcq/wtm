package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	domainstatefile "github.com/LucasPcq/wtm/internal/domain/statefile"
)

const newerSchema = "schema = 99\n"

func readBack(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertRefusedNewer(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, domainstatefile.ErrSchemaNewer) {
		t.Fatalf("error = %v, want %v", err, domainstatefile.ErrSchemaNewer)
	}
}

func TestProjectConfigAtANewerSchemaIsReadButNeverRewritten(t *testing.T) {
	dir := t.TempDir()
	content := newerSchema + minimalToml
	writeFile(t, dir, domain.ConfigFileName, content)
	path := filepath.Join(dir, domain.ConfigFileName)

	cfg, err := Load(LoadParams{StateDir: dir})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Project.Schema != 99 {
		t.Errorf("Schema = %d, want 99", cfg.Project.Schema)
	}

	assertRefusedNewer(t, WriteProjectConfig(t.Context(), WriteProjectConfigParams{StateDir: dir, Config: cfg.Project}))
	_, err = ReviseProject(ReviseProjectParams{Path: path, Current: []byte(content), Update: func(*domain.ProjectConfig) {}})
	assertRefusedNewer(t, err)
	if got := readBack(t, path); got != content {
		t.Errorf("config.toml rewritten:\n%s", got)
	}
}

func TestRunConfigAtANewerSchemaIsReadButNeverRewritten(t *testing.T) {
	dir := t.TempDir()
	content := newerSchema + "[[job]]\nname = \"web\"\ncmd = \"npm run dev\"\n"
	writeFile(t, dir, domain.RunFileName, content)

	cfg, err := LoadRun(dir)
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if cfg.Schema != 99 {
		t.Errorf("Schema = %d, want 99", cfg.Schema)
	}

	assertRefusedNewer(t, WriteRun(t.Context(), WriteRunParams{StateDir: dir, Config: cfg, Force: true}))
	if got := readBack(t, filepath.Join(dir, domain.RunFileName)); got != content {
		t.Errorf("run.toml rewritten:\n%s", got)
	}
}

func TestGlobalConfigAtANewerSchemaIsNeverRewritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), domain.GlobalConfigFile)
	content := newerSchema + "shell = \"fish\"\n"
	writeFile(t, filepath.Dir(path), domain.GlobalConfigFile, content)

	assertRefusedNewer(t, writeGlobalAt(t.Context(), writeGlobalAtParams{Path: path, Answers: domain.InitGlobalAnswers{Shell: domain.ShellZsh}}))
	if got := readBack(t, path); got != content {
		t.Errorf("global config rewritten:\n%s", got)
	}
}

func TestPendingRemovalsAtANewerSchemaAreReadButNeverRewritten(t *testing.T) {
	dir := t.TempDir()
	content := newerSchema + "[[pending]]\njob = \"db\"\nworktree = \"feat-a\"\n"
	writeFile(t, dir, domain.PendingRemovalsFileName, content)

	if got := LoadPendingRemovals(dir); len(got) != 1 {
		t.Fatalf("LoadPendingRemovals = %v, want the one entry", got)
	}
	err := UpdatePendingRemovals(t.Context(), UpdatePendingRemovalsParams{StateDir: dir, Update: func([]domain.NamespaceRef) []domain.NamespaceRef { return nil }})
	assertRefusedNewer(t, err)
	if got := readBack(t, filepath.Join(dir, domain.PendingRemovalsFileName)); got != content {
		t.Errorf("pending-removals.toml rewritten:\n%s", got)
	}
}

func TestStateFilesAreWrittenWithoutASchemaKeyAtSchemaOne(t *testing.T) {
	dir := t.TempDir()
	if err := WriteRun(t.Context(), WriteRunParams{StateDir: dir}); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	if err := WriteProject(t.Context(), WriteProjectParams{StateDir: dir, Answers: domain.InitProjectAnswers{BasePath: "../wt", BaseBranch: "main"}}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	ref := domain.NamespaceRef{Job: "db", Worktree: "feat-a"}
	if err := UpdatePendingRemovals(t.Context(), UpdatePendingRemovalsParams{StateDir: dir, Update: func([]domain.NamespaceRef) []domain.NamespaceRef { return []domain.NamespaceRef{ref} }}); err != nil {
		t.Fatalf("UpdatePendingRemovals: %v", err)
	}
	for _, name := range []string{domain.RunFileName, domain.ConfigFileName, domain.PendingRemovalsFileName} {
		if got := readBack(t, filepath.Join(dir, name)); strings.Contains(got, "schema =") {
			t.Errorf("%s carries a schema key at schema 1:\n%s", name, got)
		}
	}
}

func TestReviseProjectChangesOnlyWhatItIsAskedTo(t *testing.T) {
	dir := t.TempDir()
	revised, err := ReviseProject(ReviseProjectParams{Path: filepath.Join(dir, domain.ConfigFileName), Current: []byte(minimalToml), Update: func(project *domain.ProjectConfig) {
		project.Wizard.Remembered = map[string]string{domain.RememberIsolation: "verbatim"}
	}})
	if err != nil {
		t.Fatalf("ReviseProject: %v", err)
	}
	writeFile(t, dir, domain.ConfigFileName, string(revised))

	project, err := LoadProjectRaw(dir)
	if err != nil {
		t.Fatalf("LoadProjectRaw: %v", err)
	}
	if project.Worktrees.BasePath != "" || project.Worktrees.BaseBranch != "" {
		t.Errorf("defaults written in: %+v", project.Worktrees)
	}
	if project.Wizard.Remembered[domain.RememberIsolation] != "verbatim" {
		t.Errorf("remembered = %v", project.Wizard.Remembered)
	}
}

func TestReviseProjectNeedsAFile(t *testing.T) {
	_, err := ReviseProject(ReviseProjectParams{Path: domain.ConfigFileName, Update: func(*domain.ProjectConfig) {}})
	if !errors.Is(err, domain.ErrConfigNotFound) {
		t.Errorf("error = %v, want %v", err, domain.ErrConfigNotFound)
	}
}

func TestUpdatePendingRemovalsRemovesTheFileOnceEmpty(t *testing.T) {
	dir := t.TempDir()
	ref := domain.NamespaceRef{Job: "db", Worktree: "feat-a"}
	queue := func(refs []domain.NamespaceRef) {
		t.Helper()
		if err := UpdatePendingRemovals(t.Context(), UpdatePendingRemovalsParams{StateDir: dir, Update: func([]domain.NamespaceRef) []domain.NamespaceRef { return refs }}); err != nil {
			t.Fatalf("UpdatePendingRemovals: %v", err)
		}
	}
	queue([]domain.NamespaceRef{ref})
	queue(nil)
	if _, err := os.Stat(filepath.Join(dir, domain.PendingRemovalsFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("pending-removals.toml still there: %v", err)
	}
}
