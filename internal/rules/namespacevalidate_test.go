package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func sharedJob(name string, namespace *domain.JobNamespaceConfig) domain.JobConfig {
	return domain.JobConfig{
		Name: name, Kind: domain.JobKindService, Cmd: "docker compose up " + name,
		Scope: domain.JobScopeShared, Namespace: namespace,
	}
}

func TestValidateNamespacesRefusesOneWithoutACreateCommand(t *testing.T) {
	errs := ValidateNamespaces(domain.RunConfig{
		Jobs: []domain.JobConfig{sharedJob("db", &domain.JobNamespaceConfig{Name: "app_{worktree}"})},
	})
	if len(errs) != 1 || !strings.Contains(errs[0], "create") {
		t.Errorf("errs = %v, want one mentioning attach", errs)
	}
}

func TestValidateNamespacesRefusesNamespaceOnPerWorktreeJob(t *testing.T) {
	job := sharedJob("db", &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "true"})
	job.Scope = domain.JobScopePerWorktree
	errs := ValidateNamespaces(domain.RunConfig{Jobs: []domain.JobConfig{job}})
	if len(errs) != 1 || !strings.Contains(errs[0], "shared") {
		t.Errorf("errs = %v, want one mentioning shared", errs)
	}
}

func TestValidateNamespacesRefusesUnknownPlaceholder(t *testing.T) {
	errs := ValidateNamespaces(domain.RunConfig{
		Jobs: []domain.JobConfig{sharedJob("db", &domain.JobNamespaceConfig{Name: "app_{branch}", Create: "true"})},
	})
	if len(errs) != 1 || !strings.Contains(errs[0], "{branch}") {
		t.Errorf("errs = %v, want one naming {branch}", errs)
	}
}

// A shared job with no namespace is a valid answer: shared for good, one instance
// and one set of data — the keycloak-with-one-realm case.
func TestValidateNamespacesAcceptsSharedJobWithoutNamespace(t *testing.T) {
	if errs := ValidateNamespaces(domain.RunConfig{Jobs: []domain.JobConfig{sharedJob("db", nil)}}); len(errs) != 0 {
		t.Errorf("errs = %v, want none", errs)
	}
}

func TestValidateNamespacesRefusesUnknownScope(t *testing.T) {
	job := sharedJob("db", nil)
	job.Scope = domain.JobScope("global")
	errs := ValidateNamespaces(domain.RunConfig{Jobs: []domain.JobConfig{job}})
	if len(errs) != 1 || !strings.Contains(errs[0], "global") {
		t.Errorf("errs = %v, want one naming the unknown scope", errs)
	}
}

func TestValidateNamespacesAcceptsAPlainConfig(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev"}}}
	if errs := ValidateNamespaces(cfg); len(errs) != 0 {
		t.Errorf("errs = %v, want none: a config predating scope must read unchanged", errs)
	}
}
