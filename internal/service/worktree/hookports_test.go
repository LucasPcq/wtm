package worktree

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func writeRunConfig(t *testing.T, stateDir, body string) {
	t.Helper()
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, domain.RunFileName), []byte(body), 0o644); err != nil {
		t.Fatalf("write run config: %v", err)
	}
}

func branchEnv(t *testing.T, repo ordinalRepo, branch string) map[string]string {
	t.Helper()
	repo.ensure(t, branch)
	env, err := BranchEnv(t.Context(), WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir, Branch: branch})
	if err != nil {
		t.Fatalf("BranchEnv(%s): %v", branch, err)
	}
	return env
}

// A hook is not a job, so it gets the ports no job disputes — the templated
// compose recipe `"${DB_PORT}:5432"` is what an on_clean tears down.
func TestBranchEnvGivesHookTheWorktreesPorts(t *testing.T) {
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, `
[[job]]
name = "db"
kind = "service"
cmd = "docker compose up"
ports = { DB_PORT = 5432 }
`)
	repo.addWorktree(t, "feat/x")

	env := branchEnv(t, repo, "feat/x")

	if env[domain.EnvPortOffset] != "10" {
		t.Fatalf("%s = %q, want %q", domain.EnvPortOffset, env[domain.EnvPortOffset], "10")
	}
	if env["DB_PORT"] != "5442" {
		t.Errorf("DB_PORT = %q, want %q", env["DB_PORT"], "5442")
	}
}

func TestBranchEnvMainCheckoutKeepsDeclaredPorts(t *testing.T) {
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, `
[[job]]
name = "db"
kind = "service"
cmd = "docker compose up"
ports = { DB_PORT = 5432 }
`)

	if got := branchEnv(t, repo, "main")["DB_PORT"]; got != "5432" {
		t.Errorf("DB_PORT = %q, want %q", got, "5432")
	}
}

// Two jobs naming PORT leave it unresolved rather than resolved to one of them.
func TestBranchEnvLeavesAmbiguousPortOut(t *testing.T) {
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, `
[[job]]
name = "web"
kind = "service"
cmd = "pnpm dev"
ports = { PORT = 3000 }

[[job]]
name = "api"
kind = "service"
cmd = "pnpm api"
ports = { PORT = 8080, API_PORT = 9000 }
`)
	repo.addWorktree(t, "feat/x")

	env := branchEnv(t, repo, "feat/x")

	if _, declared := env["PORT"]; declared {
		t.Errorf("PORT = %q, want it left out", env["PORT"])
	}
	if env["API_PORT"] != "9010" {
		t.Errorf("API_PORT = %q, want %q", env["API_PORT"], "9010")
	}
}

// A run.toml that cannot be read costs the ports, never the identity.
func TestBranchEnvKeepsIdentityWhenRunConfigIsUnreadable(t *testing.T) {
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, "[[job]\nname = ")
	repo.addWorktree(t, "feat/x")

	env := branchEnv(t, repo, "feat/x")

	if env[domain.EnvBranch] != "feat/x" {
		t.Errorf("%s = %q, want %q", domain.EnvBranch, env[domain.EnvBranch], "feat/x")
	}
	if env[domain.EnvComposeProjectName] == "" {
		t.Errorf("%s is empty", domain.EnvComposeProjectName)
	}
}

// End to end: what run.toml declares reaches the on_clean of an isolated
// worktree, so a templated compose file no longer resolves ${DB_PORT} to nothing.
func TestRunCleanHooksSeesResolvedPorts(t *testing.T) {
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, `
[[job]]
name = "db"
kind = "service"
cmd = "docker compose up"
ports = { DB_PORT = 5432 }
`)
	path := repo.addWorktree(t, "feat/x")
	recordIsolation(t, repo, "feat/x", domain.IsolationIsolated)
	repo.ensure(t, "feat/x")
	marker := filepath.Join(t.TempDir(), "seen")

	var out bytes.Buffer
	if err := RunCleanHooks(t.Context(), domain.CleanHooksParams{
		ProjectDir:   repo.dir,
		StateDir:     repo.stateDir,
		WorktreePath: path,
		Branch:       "feat/x",
		Hooks:        []domain.HookCommand{{Cmd: "printenv DB_PORT > " + marker}},
		Output:       &out,
	}); err != nil {
		t.Fatalf("RunCleanHooks: %v", err)
	}

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("hook left no marker: %v", err)
	}
	if strings.TrimSpace(string(got)) != "5442" {
		t.Errorf("hook saw DB_PORT=%q, want %q", strings.TrimSpace(string(got)), "5442")
	}
}
