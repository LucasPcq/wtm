package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

const composeRunConfig = `
[[job]]
name = "infra"
kind = "service"
scope = "shared"
cmd = "docker compose up -d"
cwd = "infra"
`

func mainProjectSlug(repo ordinalRepo) string {
	return rules.WorktreeSlug(filepath.Base(repo.dir))
}

// The main checkout hosts the shared stack: a name following its branch would
// start a second stack on every checkout and orphan the first.
func TestBranchEnvNamesTheMainComposeProjectAfterTheRepository(t *testing.T) {
	t.Setenv(domain.EnvComposeProjectName, "launched-from-another-worktree")
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, composeRunConfig)

	if got := branchEnv(t, repo, "main")[domain.EnvComposeProjectName]; got != mainProjectSlug(repo) {
		t.Errorf("%s on main = %q, want %q", domain.EnvComposeProjectName, got, mainProjectSlug(repo))
	}

	git(t, repo.dir, "checkout", "-b", "chore/dev-multi-worktree")
	if got := branchEnv(t, repo, "chore/dev-multi-worktree")[domain.EnvComposeProjectName]; got != mainProjectSlug(repo) {
		t.Errorf("%s after a branch switch = %q, want %q", domain.EnvComposeProjectName, got, mainProjectSlug(repo))
	}
}

// Compose reads the .env of the directory it runs from, so that is where the
// main's own name is looked up — not the shell's environment.
func TestBranchEnvReadsTheMainComposeProjectFromItsEnvFile(t *testing.T) {
	t.Setenv(domain.EnvComposeProjectName, "launched-from-another-worktree")
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, composeRunConfig)
	if err := os.MkdirAll(filepath.Join(repo.dir, "infra"), 0o755); err != nil {
		t.Fatalf("mkdir infra: %v", err)
	}
	writeEnv(t, filepath.Join(repo.dir, "infra"), "COMPOSE_PROJECT_NAME=kresus\n")

	if got := branchEnv(t, repo, "main")[domain.EnvComposeProjectName]; got != "kresus" {
		t.Errorf("%s = %q, want %q", domain.EnvComposeProjectName, got, "kresus")
	}
}

func TestBranchEnvKeepsTheBranchInALinkedWorktreesComposeProject(t *testing.T) {
	t.Setenv(domain.EnvComposeProjectName, "")
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, composeRunConfig)
	repo.addWorktree(t, "feat/x")

	want := rules.ComposeProjectName(rules.ComposeProjectNameParams{Project: filepath.Base(repo.dir), Worktree: "feat-x"})
	if got := branchEnv(t, repo, "feat/x")[domain.EnvComposeProjectName]; got != want {
		t.Errorf("%s = %q, want %q", domain.EnvComposeProjectName, got, want)
	}
}

// The owned write and the jobs must agree on the main's name, or a compose run
// outside wtm lands on another project than the one wtm started.
func TestResolveEnvPortsWritesTheMainsStableComposeProject(t *testing.T) {
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, composeRunConfig)
	git(t, repo.dir, "checkout", "-b", "chore/dev-multi-worktree")

	resolved, err := ResolveEnvPorts(ResolveEnvPortsParams{
		ProjectDir:   repo.dir,
		StateDir:     repo.stateDir,
		Branch:       "chore/dev-multi-worktree",
		WorktreePath: repo.dir,
		EnvFiles:     []domain.EnvFile{{Target: "infra/.env"}},
	})
	if err != nil {
		t.Fatalf("ResolveEnvPorts: %v", err)
	}
	if len(resolved.Owned) != 1 || resolved.Owned[0].Value != mainProjectSlug(repo) {
		t.Errorf("owned = %+v, want one %s = %q", resolved.Owned, domain.EnvComposeProjectName, mainProjectSlug(repo))
	}
}
