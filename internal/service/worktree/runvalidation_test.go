package worktree

import (
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
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
	path := repo.addWorktree(t, "feat/a")
	repo.ensure(t, "feat/a")
	resolved, err := ResolveEnvPorts(t.Context(), ResolveEnvPortsParams{
		ProjectDir:   repo.dir,
		StateDir:     repo.stateDir,
		Branch:       "feat/a",
		WorktreePath: path,
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

const mainEnvValueConfig = `
[[job]]
name = "kc"
kind = "service"
cmd = "run-kc"

[[env]]
file = ".env"
key = "KEYCLOAK_REALM"
job = "kc"
value = "acme-{worktree}"
`

func resolveMain(t *testing.T, repo ordinalRepo) envsvc.EnvPortsParams {
	t.Helper()
	resolved, err := ResolveEnvPorts(t.Context(), ResolveEnvPortsParams{
		ProjectDir:   repo.dir,
		StateDir:     repo.stateDir,
		Branch:       "main",
		WorktreePath: repo.dir,
		EnvFiles:     []domain.EnvFile{{Target: ".env", Template: ".env.example"}},
	})
	if err != nil {
		t.Fatalf("ResolveEnvPorts: %v", err)
	}
	return resolved
}

// The main checkout's realms and databases are its own: wtm writes no [[env]]
// value there, and the keys stay plain keys the reconciliation compares.
func TestResolveEnvPortsWritesNoEnvValueIntoMain(t *testing.T) {
	repo := newOrdinalRepo(t)
	globaldir.Isolate(t)
	writeRunConfig(t, repo.stateDir, mainEnvValueConfig)
	writeFile(t, repo.dir, ".env.example", "KEYCLOAK_REALM=acme\n")
	writeFile(t, repo.dir, ".env", "KEYCLOAK_REALM=acme-local\n")

	resolved := resolveMain(t, repo)
	if len(resolved.ValueLinks) != 0 || len(resolved.Owned) != 0 {
		t.Errorf("value links = %+v, owned = %+v; want nothing wtm writes into main", resolved.ValueLinks, resolved.Owned)
	}
}

// A value wtm stamped into main before it stopped goes back to the template's.
func TestResolveEnvPortsPutsBackWhatWasStampedIntoMain(t *testing.T) {
	repo := newOrdinalRepo(t)
	globaldir.Isolate(t)
	writeRunConfig(t, repo.stateDir, mainEnvValueConfig)
	writeFile(t, repo.dir, ".env.example", "KEYCLOAK_REALM=acme\n")
	writeFile(t, repo.dir, ".env", "KEYCLOAK_REALM=acme-main\n")

	resolved := resolveMain(t, repo)
	want := []domain.EnvOwnedEntry{{File: ".env", Key: "KEYCLOAK_REALM", Value: "acme"}}
	if !slices.Equal(resolved.Owned, want) {
		t.Errorf("owned = %+v, want %+v", resolved.Owned, want)
	}
}
