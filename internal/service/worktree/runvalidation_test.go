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
	_, err := ResolveEnvPorts(ResolveEnvPortsParams{
		ProjectDir:   repo.dir,
		StateDir:     repo.stateDir,
		Branch:       "main",
		WorktreePath: repo.dir,
		EnvFiles:     []domain.EnvFile{{Target: ".env"}},
		Global:       global,
	})
	return err
}

// An [[env]] link is held to the same file check as an [[env_port]]: a key
// written into a .env nothing provisions is a promise wtm cannot keep.
func TestResolveEnvPortsRefusesAnEnvValueOnAnUnconfiguredFile(t *testing.T) {
	repo := newOrdinalRepo(t)
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
`)
	err := resolveErr(t, repo, domain.GlobalConfig{})
	if err == nil || !strings.Contains(err.Error(), ".env.local") {
		t.Fatalf("want a refusal naming .env.local, got %v", err)
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
