package worktree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

const namedRunConfig = `
addressing = "names"

[[job]]
name = "api-dev"
kind = "service"
cmd = "pnpm dev"
ports = { PORT = 4001 }
url = { port = "PORT" }

[[env_port]]
file = ".env"
key = "VITE_API_URL"
job = "api-dev"
port = "PORT"
`

func writeEnv(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(body), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
}

func planOf(t *testing.T, repo ordinalRepo, branch, path string) domain.EnvPortPlan {
	t.Helper()
	globaldir.Isolate(t)
	plan, err := EnvPortPlanFor(t.Context(), ResolveEnvPortsParams{
		ProjectDir:   repo.dir,
		StateDir:     repo.stateDir,
		Branch:       branch,
		WorktreePath: path,
		EnvFiles:     []domain.EnvFile{{Target: ".env"}},
	})
	if err != nil {
		t.Fatalf("EnvPortPlanFor(%s): %v", branch, err)
	}
	return plan
}

// The main checkout is the standing case: nothing ever writes its .env, so a
// project publishing names finds it still spelled by port.
func TestEnvPortPlanForSeesTheMainCheckoutStillOnPorts(t *testing.T) {
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, namedRunConfig)
	writeEnv(t, repo.dir, "VITE_API_URL=http://localhost:4001\n")

	plan := planOf(t, repo, "main", repo.dir)

	if got := rules.PendingOriginRewrites(plan); got != 1 {
		t.Fatalf("pending = %d, want the one origin still spelled by port: %+v", got, plan.Entries)
	}
}

func TestEnvPortPlanForReportsNothingOnceAligned(t *testing.T) {
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, namedRunConfig)
	writeEnv(t, repo.dir, "VITE_API_URL=http://localhost:4001\n")

	// The value the pass would write, written: what `wtm env main` does, and the
	// state every surface must then read as aligned.
	writeEnv(t, repo.dir, "VITE_API_URL="+planOf(t, repo, "main", repo.dir).Entries[0].NewValue+"\n")

	plan := planOf(t, repo, "main", repo.dir)

	if got := rules.PendingOriginRewrites(plan); got != 0 {
		t.Errorf("pending = %d, want none for an aligned .env: %+v", got, plan.Entries)
	}
}

// A project declaring no link resolves to an empty plan rather than an error,
// which is what lets a surface probe unconditionally.
func TestEnvPortPlanForIsEmptyWithoutLinks(t *testing.T) {
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, `
[[job]]
name = "api-dev"
kind = "service"
cmd = "pnpm dev"
ports = { PORT = 4001 }
`)

	if plan := planOf(t, repo, "main", repo.dir); len(plan.Entries) != 0 {
		t.Errorf("entries = %+v, want none", plan.Entries)
	}
}

// The name is the address whatever the .env still spells: hiding it behind the
// port made the one URL a reader came for disappear. The .env out of step is
// said once, in the note.
func TestRunAddressesForServesTheNameOfAnUnsettledWorktree(t *testing.T) {
	globaldir.Isolate(t)
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, namedRunConfig)
	writeEnv(t, repo.dir, "VITE_API_URL=http://localhost:4001\n")

	answer := RunAddressesFor(t.Context(), RunAddressesForParams{
		ProjectDir: repo.dir,
		StateDir:   repo.stateDir,
		RunConfig:  runConfigOf(t, repo),
		Branches:   []string{"main"},
		EnvFiles:   []domain.EnvFile{{Target: ".env"}},
		ProxyPort:  11080,
	})

	url := answer.ByBranch["main"]["api-dev"].URL
	if !strings.Contains(url, ".localhost:11080") {
		t.Errorf("url = %q, want the published name", url)
	}
	if answer.Notes["main"] == "" {
		t.Error("a .env out of step with the names it publishes must be said")
	}
}

func runConfigOf(t *testing.T, repo ordinalRepo) domain.RunConfig {
	t.Helper()
	cfg, err := config.LoadRun(repo.stateDir)
	if err != nil {
		t.Fatalf("load run config: %v", err)
	}
	return cfg
}
