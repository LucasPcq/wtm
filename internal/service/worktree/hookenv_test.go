package worktree

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

const composeJobConfig = `
[[job]]
name = "db"
kind = "service"
cmd = "docker compose up -d"
ports = { DB_PORT = 5432 }
`

func hookSaw(t *testing.T, repo ordinalRepo, branch, path string) map[string]string {
	t.Helper()
	if HookEnvPending(WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir, Branch: branch}) {
		repo.ensure(t, branch)
	}
	marker := filepath.Join(t.TempDir(), "env")
	var out bytes.Buffer
	if err := RunCleanHooks(domain.CleanHooksParams{
		ProjectDir:   repo.dir,
		StateDir:     repo.stateDir,
		WorktreePath: path,
		Branch:       branch,
		Hooks:        []domain.HookCommand{{Cmd: "env > " + marker}},
		Output:       &out,
	}); err != nil {
		t.Fatalf("RunCleanHooks: %v", err)
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("hook left no marker: %v", err)
	}
	seen := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, found := strings.Cut(line, "=")
		if found {
			seen[key] = value
		}
	}
	return seen
}

func unsetComposeProject(t *testing.T) {
	t.Helper()
	t.Setenv(domain.EnvComposeProjectName, "")
	if err := os.Unsetenv(domain.EnvComposeProjectName); err != nil {
		t.Fatalf("unsetenv: %v", err)
	}
}

func recordIsolation(t *testing.T, repo ordinalRepo, branch string, isolation domain.Isolation) {
	t.Helper()
	ref := WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir, Branch: branch}
	if err := SetIsolation(SetIsolationParams{Ref: ref, Isolation: isolation}); err != nil {
		t.Fatalf("SetIsolation: %v", err)
	}
}

func assertNoRunEnv(t *testing.T, seen map[string]string) {
	t.Helper()
	for _, key := range append([]string{"DB_PORT"}, domain.WorktreeScopedEnv...) {
		if value, set := seen[key]; set {
			t.Errorf("hook saw %s=%q, want the v0.27 environment", key, value)
		}
	}
}

func TestHookOfAWorktreeThatNeverChoseIsolationGetsNoRunVariables(t *testing.T) {
	unsetComposeProject(t)
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, composeJobConfig)
	path := repo.addWorktree(t, "feat/x")

	assertNoRunEnv(t, hookSaw(t, repo, "feat/x", path))
	if _, err := os.Stat(filepath.Join(rules.WorktreeMetaDir(repo.stateDir, "feat/x"), domain.MetaFileName)); !os.IsNotExist(err) {
		t.Errorf("a hook allocated an ordinal for a worktree the run variables do not apply to (stat err = %v)", err)
	}
}

func TestHookGetsNoRunVariablesWithoutAComposeJob(t *testing.T) {
	unsetComposeProject(t)
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, `
[[job]]
name = "web"
kind = "service"
cmd = "pnpm dev"
ports = { DB_PORT = 3000 }
`)
	path := repo.addWorktree(t, "feat/x")
	recordIsolation(t, repo, "feat/x", domain.IsolationIsolated)

	assertNoRunEnv(t, hookSaw(t, repo, "feat/x", path))
	if got := repo.meta(t, "feat/x").Ordinal; got != 0 {
		t.Errorf("ordinal = %d, want none allocated by a hook", got)
	}
}

func TestHookGetsNoRunVariablesWithoutRunConfig(t *testing.T) {
	unsetComposeProject(t)
	repo := newOrdinalRepo(t)
	path := repo.addWorktree(t, "feat/x")
	recordIsolation(t, repo, "feat/x", domain.IsolationIsolated)

	assertNoRunEnv(t, hookSaw(t, repo, "feat/x", path))
}

func TestHookOfAnIsolatedWorktreeWithAComposeJobGetsItsOwnVariables(t *testing.T) {
	t.Setenv(domain.EnvComposeProjectName, "launched-from-another-worktree")
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, composeJobConfig)
	path := repo.addWorktree(t, "feat/x")
	recordIsolation(t, repo, "feat/x", domain.IsolationIsolated)

	seen := hookSaw(t, repo, "feat/x", path)

	want := rules.ComposeProjectName(rules.ComposeProjectNameParams{Project: filepath.Base(repo.dir), Worktree: "feat-x"})
	if seen[domain.EnvComposeProjectName] != want {
		t.Errorf("%s = %q, want %q", domain.EnvComposeProjectName, seen[domain.EnvComposeProjectName], want)
	}
	if seen["DB_PORT"] != "5442" || seen[domain.EnvOrdinal] != "1" {
		t.Errorf("DB_PORT = %q, %s = %q, want 5442 and 1", seen["DB_PORT"], domain.EnvOrdinal, seen[domain.EnvOrdinal])
	}
}

func TestHookComposeProjectComesFromTheWorktreesEnvFile(t *testing.T) {
	unsetComposeProject(t)
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, composeJobConfig)
	path := repo.addWorktree(t, "feat/x")
	recordIsolation(t, repo, "feat/x", domain.IsolationIsolated)
	writeEnv(t, path, "COMPOSE_PROJECT_NAME=kept-by-hand\n")

	if got := hookSaw(t, repo, "feat/x", path)[domain.EnvComposeProjectName]; got != "kept-by-hand" {
		t.Errorf("%s = %q, want the .env's %q", domain.EnvComposeProjectName, got, "kept-by-hand")
	}
}

// A linked worktree's name is derived from the worktree itself, never from the
// shell that launched the command.
func TestBranchEnvIgnoresTheCallersComposeProject(t *testing.T) {
	t.Setenv(domain.EnvComposeProjectName, "launched-from-another-worktree")
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, composeRunConfig)
	repo.addWorktree(t, "feat/x")

	want := rules.ComposeProjectName(rules.ComposeProjectNameParams{Project: filepath.Base(repo.dir), Worktree: "feat-x"})
	if got := branchEnv(t, repo, "feat/x")[domain.EnvComposeProjectName]; got != want {
		t.Errorf("%s = %q, want %q", domain.EnvComposeProjectName, got, want)
	}
}

// The choice is recorded by Create itself, before any hook can run: the flow
// runs on_create right after, and the first hook gets the stack the worktree
// will run under.
func TestCreateRecordsIsolationBeforeItsHooksRun(t *testing.T) {
	repo := newOrdinalRepo(t)
	var cfg domain.Config
	cfg.Project.Worktrees.BasePath = t.TempDir()

	if _, err := Create(domain.CreateParams{
		ProjectDir: repo.dir,
		StateDir:   repo.stateDir,
		Branch:     "feat/x",
		FromBranch: "main",
		Config:     cfg,
		Isolation:  domain.IsolationIsolated,
		SkipHooks:  true,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got := RecordedIsolation(WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir, Branch: "feat/x"}); got != domain.IsolationIsolated {
		t.Errorf("recorded isolation = %q, want %q", got, domain.IsolationIsolated)
	}
}
