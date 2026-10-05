package worktree

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

func resolveErr(t *testing.T, repo ordinalRepo, global domain.GlobalConfig) error {
	t.Helper()
	globaldir.Isolate(t)
	_, err := ResolveEnvPorts(t.Context(), ResolveEnvPortsParams{
		ProjectDir:   repo.dir,
		StateDir:     repo.stateDir,
		Branch:       "main",
		WorktreePath: repo.dir,
		EnvFiles:     []domain.EnvFile{{Target: ".env"}},
		Global:       global,
	})
	return err
}

// A link on a .env nothing provisions can only fail to apply: it is left out,
// and the links on configured files resolve as if it were not there.
func TestResolveEnvPortsLeavesOutALinkOnAnUnconfiguredFile(t *testing.T) {
	repo := newOrdinalRepo(t)
	globaldir.Isolate(t)
	writeRunConfig(t, repo.stateDir, `
[[job]]
name = "kc"
kind = "service"
cmd = "run-kc"

[[env]]
file = ".env.local"
key = "REALM"
job = "kc"
value = "{worktree}"

[[env]]
file = ".env"
key = "KC_NAME"
job = "kc"
value = "{worktree}"
`)
	resolved, err := ResolveEnvPorts(t.Context(), ResolveEnvPortsParams{
		ProjectDir:   repo.dir,
		StateDir:     repo.stateDir,
		Branch:       "main",
		WorktreePath: repo.dir,
		EnvFiles:     []domain.EnvFile{{Target: ".env"}},
	})
	if err != nil {
		t.Fatalf("ResolveEnvPorts: %v", err)
	}
	if len(resolved.ValueLinks) != 1 || resolved.ValueLinks[0].File != ".env" {
		t.Errorf("value links = %+v, want the configured one alone", resolved.ValueLinks)
	}
	for _, entry := range resolved.Owned {
		if entry.File == ".env.local" {
			t.Errorf("owned = %+v, want nothing written into the unconfigured file", resolved.Owned)
		}
	}
}

func TestResolveEnvPortsRefusesAProxyPortOutOfRange(t *testing.T) {
	repo := newOrdinalRepo(t)
	writeRunConfig(t, repo.stateDir, namedRunConfig)
	writeEnv(t, repo.dir, "VITE_API_URL=http://localhost:4001\n")

	err := resolveErr(t, repo, domain.GlobalConfig{Proxy: domain.ProxyConfig{Port: 70000}})
	if err == nil || !strings.Contains(err.Error(), "70000") {
		t.Fatalf("want a refusal naming the port, got %v", err)
	}
}
