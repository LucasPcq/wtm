package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

// The config a run loaded carries its defaults and nothing written since: a
// rewrite from it froze the first and erased the second.
func TestSetBasePathChangesOnlyBasePathOnDisk(t *testing.T) {
	globaldir.Isolate(t)
	repo := newOrdinalRepo(t)
	path := filepath.Join(repo.stateDir, domain.ConfigFileName)
	if err := os.MkdirAll(repo.stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[worktrees]\nbase_path = \"../.trees\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(config.LoadParams{StateDir: repo.stateDir})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[worktrees]\nbase_path = \"../.trees\"\n\n[wizard.remembered]\nisolation = \"verbatim\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SetBasePath(t.Context(), SetBasePathParams{
		ProjectDir:       repo.dir,
		StateDir:         repo.stateDir,
		PreviousBasePath: loaded.Project.Worktrees.BasePath,
		BasePath:         "../elsewhere",
	}); err != nil {
		t.Fatalf("SetBasePath: %v", err)
	}

	project, err := config.LoadProjectRaw(repo.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if project.Worktrees.BasePath != "../elsewhere" {
		t.Errorf("base_path = %q", project.Worktrees.BasePath)
	}
	if project.Worktrees.BaseBranch != "" || project.Env.Strategy != "" {
		t.Errorf("defaults written in: base_branch %q, strategy %q", project.Worktrees.BaseBranch, project.Env.Strategy)
	}
	if project.Wizard.Remembered[domain.RememberIsolation] != "verbatim" {
		t.Errorf("remembered = %v, the answer written since the load is lost", project.Wizard.Remembered)
	}
}
