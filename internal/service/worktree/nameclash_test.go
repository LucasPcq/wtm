package worktree

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

const oneJobRunConfig = `
[[job]]
name = "api"
kind = "service"
cmd = "pnpm dev"
`

func createIn(t *testing.T, repo ordinalRepo, base, branch string) error {
	t.Helper()
	cfg := domain.Config{Project: domain.ProjectConfig{Worktrees: domain.WorktreesConfig{BasePath: base, BaseBranch: "main"}}}
	_, err := Create(domain.CreateParams{
		ProjectDir: repo.dir,
		StateDir:   repo.stateDir,
		Branch:     branch,
		FromBranch: "main",
		Config:     cfg,
	})
	return err
}

// feat/x and feat.x would run under one compose project, one namespace and
// one proxy host: the second is refused before git creates anything.
func TestCreateRefusesANameALiveWorktreeDerivesToo(t *testing.T) {
	globaldir.Isolate(t)
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, oneJobRunConfig)
	base := t.TempDir()
	if err := createIn(t, repo, base, "feat/x"); err != nil {
		t.Fatalf("create feat/x: %v", err)
	}

	err := createIn(t, repo, base, "feat.x")
	if !errors.Is(err, domain.ErrWorktreeNameTaken) {
		t.Fatalf("create feat.x = %v, want ErrWorktreeNameTaken", err)
	}
	if !strings.Contains(err.Error(), "feat.x would share its name with feat/x (feat-x)") {
		t.Errorf("error = %q, want both branches and the shared name", err)
	}
	if rules.ExitCode(err) != domain.ExitCodeWorktreeExists {
		t.Errorf("exit code = %d, want %d", rules.ExitCode(err), domain.ExitCodeWorktreeExists)
	}
	if _, statErr := os.Stat(filepath.Join(base, "feat.x")); !os.IsNotExist(statErr) {
		t.Errorf("the worktree was created anyway: %v", statErr)
	}
}

// Without a run module no derived name is ever used, and the core creates the
// worktree exactly as it always did.
func TestCreateAllowsTheSameDerivedNameWithoutRunJobs(t *testing.T) {
	globaldir.Isolate(t)
	repo := newOrdinalRepo(t)
	base := t.TempDir()
	if err := createIn(t, repo, base, "feat/x"); err != nil {
		t.Fatalf("create feat/x: %v", err)
	}
	if err := createIn(t, repo, base, "feat.x"); err != nil {
		t.Fatalf("create feat.x without run.toml: %v", err)
	}
}

func TestRelocateRefusesToAdoptAClashingName(t *testing.T) {
	globaldir.Isolate(t)
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, oneJobRunConfig)
	base := t.TempDir()
	if err := createIn(t, repo, base, "feat/x"); err != nil {
		t.Fatalf("create feat/x: %v", err)
	}
	repo.addWorktree(t, "feat.x")

	result, err := Relocate(RelocateParams{
		ProjectDir:     repo.dir,
		StateDir:       repo.stateDir,
		Config:         domain.Config{Project: domain.ProjectConfig{Worktrees: domain.WorktreesConfig{BasePath: base, BaseBranch: "main"}}},
		TargetBasePath: base,
		BaseBranch:     "main",
	})
	if err != nil {
		t.Fatalf("Relocate: %v", err)
	}
	for _, step := range result.Steps {
		if step.Branch != "feat.x" {
			continue
		}
		if step.Status != domain.RelocateStatusBlockedName || !strings.Contains(step.Detail, "feat/x") {
			t.Fatalf("feat.x = %+v, want blocked_name naming feat/x", step)
		}
		if meta, metaErr := loadMetadata(repo.stateDir, "feat.x"); metaErr == nil && meta.CreatedAt != "" {
			t.Errorf("feat.x was adopted anyway: %+v", meta)
		}
		return
	}
	t.Fatalf("no step for feat.x in %+v", result.Steps)
}
