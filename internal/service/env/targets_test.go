package env

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

// A section [env] skipped at init has no strategy: the target added to it
// must stay live without the default strategy being written in for it.
func TestAddEnvTargetsChangesOnlyTheTargetsOnDisk(t *testing.T) {
	globaldir.Isolate(t)
	dir := t.TempDir()
	path := filepath.Join(dir, domain.ConfigFileName)
	writeTestFile(t, path, "[worktrees]\nbase_path = \"../.trees\"\n")
	writeTestFile(t, path, "[worktrees]\nbase_path = \"../.trees\"\n\n[wizard.remembered]\nisolation = \"verbatim\"\n")

	if err := AddEnvTargets(t.Context(), AddEnvTargetsParams{
		StateDir: dir,
		Targets:  []domain.EnvFile{{Target: "apps/api/.env"}},
	}); err != nil {
		t.Fatalf("AddEnvTargets: %v", err)
	}

	project, err := config.LoadProjectRaw(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Env.Files) != 1 || project.Env.Files[0].Target != "apps/api/.env" {
		t.Errorf("env files = %+v, want the added target", project.Env.Files)
	}
	if project.Worktrees.BaseBranch != "" || project.Env.Strategy != "" {
		t.Errorf("defaults written in: base_branch %q, strategy %q", project.Worktrees.BaseBranch, project.Env.Strategy)
	}
	if project.Wizard.Remembered[domain.RememberIsolation] != "verbatim" {
		t.Errorf("remembered = %v, the answer written since the load is lost", project.Wizard.Remembered)
	}
}
