package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// envValueFixture is a worktree whose .env holds the value of the main checkout,
// the state a fresh copy is in before anything reconciles it.
func envValueFixture(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func keycloakConfig() domain.RunConfig {
	return domain.RunConfig{
		Jobs: []domain.JobConfig{{
			Name:  "keycloak",
			Kind:  domain.JobKindService,
			Scope: domain.JobScopeShared,
			Ports: map[string]int{"KEYCLOAK_PORT": 8080},
			Namespace: &domain.JobNamespaceConfig{
				Name: "realm_{worktree}", Create: "true",
			},
		}},
		EnvValues: []domain.EnvValueLink{
			{File: ".env", Key: "KEYCLOAK_REALM", Job: "keycloak", Value: "{namespace}"},
		},
	}
}

// The case the table was added for: one keycloak for the repository, one realm
// per worktree. The realm key is opaque — no port, no URL — so nothing but an
// [[env]] link could ever have written it.
func TestEnvValueLinkWritesTheWorktreesSliceIntoItsEnv(t *testing.T) {
	dir := envValueFixture(t, "KEYCLOAK_URL=http://localhost:8080\nKEYCLOAK_REALM=realm_main\n")

	owned, err := rules.EnvValueWrites(rules.EnvValueWritesParams{
		Config: keycloakConfig(), Worktree: "feat-a", Ordinal: 1, Offset: 10,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if _, err := ApplyEnvPorts(EnvPortsParams{
		WorktreePath: dir,
		Owned:        owned,
		ValueLinks:   keycloakConfig().EnvValues,
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "KEYCLOAK_REALM=realm_feat-a") {
		t.Errorf("got %q, want KEYCLOAK_REALM=realm_feat-a", content)
	}
	// The shared instance answers at one address for every worktree, so the key
	// naming it is none of this table's business and must be left alone.
	if !strings.Contains(string(content), "KEYCLOAK_URL=http://localhost:8080") {
		t.Errorf("got %q, want KEYCLOAK_URL untouched", content)
	}
}

// A key wtm writes in full differs from its source by construction. Reported as
// a conflict, every `wtm env` would offer to undo the isolation it just set up.
func TestEnvValueLinkKeyIsNeitherDriftNorConflict(t *testing.T) {
	parent := []domain.EnvLine{
		{Kind: domain.EnvLinePair, Key: "KEYCLOAK_REALM", Value: "realm_main"},
	}
	child := []domain.EnvLine{
		{Kind: domain.EnvLinePair, Key: "KEYCLOAK_REALM", Value: "realm_feat-a"},
	}

	diff := rules.DiffEnv(rules.EnvDiffParams{
		Parent: parent,
		Child:  child,
		Mode:   domain.EnvModeRefresh,
		Owned:  rules.EnvValueOwnedKeys(keycloakConfig().EnvValues, ".env"),
	})

	for _, key := range diff.Entries {
		if key.Key != "KEYCLOAK_REALM" {
			continue
		}
		if key.Status != domain.EnvKeyResolved {
			t.Fatalf("status = %s, want resolved — the key is wtm's own", key.Status)
		}
		return
	}
	t.Fatal("KEYCLOAK_REALM missing from the diff")
}

// Without the link the very same key is an ordinary one, and a value differing
// from the parent is exactly the conflict wtm exists to report.
func TestEnvValueKeyStaysAConflictWhenNoLinkClaimsIt(t *testing.T) {
	diff := rules.DiffEnv(rules.EnvDiffParams{
		Parent: []domain.EnvLine{{Kind: domain.EnvLinePair, Key: "KEYCLOAK_REALM", Value: "realm_main"}},
		Child:  []domain.EnvLine{{Kind: domain.EnvLinePair, Key: "KEYCLOAK_REALM", Value: "realm_feat-a"}},
		Mode:   domain.EnvModeRefresh,
	})

	for _, key := range diff.Entries {
		if key.Key != "KEYCLOAK_REALM" {
			continue
		}
		if key.Status != domain.EnvKeyConflict {
			t.Fatalf("status = %s, want conflict", key.Status)
		}
		return
	}
	t.Fatal("KEYCLOAK_REALM missing from the diff")
}
