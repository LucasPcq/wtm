package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func sharedJob(name string, tenant *domain.JobTenantConfig) domain.JobConfig {
	return domain.JobConfig{
		Name: name, Kind: domain.JobKindService, Cmd: "docker compose up " + name,
		Scope: domain.JobScopeShared, Tenant: tenant,
	}
}

func TestValidateTenantsRefusesTenantWithoutAttach(t *testing.T) {
	errs := ValidateTenants(domain.RunConfig{
		Jobs: []domain.JobConfig{sharedJob("db", &domain.JobTenantConfig{Name: "app_{worktree}"})},
	})
	if len(errs) != 1 || !strings.Contains(errs[0], "attach") {
		t.Errorf("errs = %v, want one mentioning attach", errs)
	}
}

func TestValidateTenantsRefusesTenantOnPerWorktreeJob(t *testing.T) {
	job := sharedJob("db", &domain.JobTenantConfig{Name: "app_{worktree}", Attach: "true"})
	job.Scope = domain.JobScopePerWorktree
	errs := ValidateTenants(domain.RunConfig{Jobs: []domain.JobConfig{job}})
	if len(errs) != 1 || !strings.Contains(errs[0], "shared") {
		t.Errorf("errs = %v, want one mentioning shared", errs)
	}
}

func TestValidateTenantsRefusesUnknownPlaceholder(t *testing.T) {
	errs := ValidateTenants(domain.RunConfig{
		Jobs: []domain.JobConfig{sharedJob("db", &domain.JobTenantConfig{Name: "app_{branch}", Attach: "true"})},
	})
	if len(errs) != 1 || !strings.Contains(errs[0], "{branch}") {
		t.Errorf("errs = %v, want one naming {branch}", errs)
	}
}

// A shared job with no tenant is a valid answer: shared for good, one instance
// and one set of data — the keycloak-with-one-realm case.
func TestValidateTenantsAcceptsSharedJobWithoutTenant(t *testing.T) {
	if errs := ValidateTenants(domain.RunConfig{Jobs: []domain.JobConfig{sharedJob("db", nil)}}); len(errs) != 0 {
		t.Errorf("errs = %v, want none", errs)
	}
}

func TestValidateTenantsRefusesUnknownScope(t *testing.T) {
	job := sharedJob("db", nil)
	job.Scope = domain.JobScope("global")
	errs := ValidateTenants(domain.RunConfig{Jobs: []domain.JobConfig{job}})
	if len(errs) != 1 || !strings.Contains(errs[0], "global") {
		t.Errorf("errs = %v, want one naming the unknown scope", errs)
	}
}

func TestValidateTenantsAcceptsAPlainConfig(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev"}}}
	if errs := ValidateTenants(cfg); len(errs) != 0 {
		t.Errorf("errs = %v, want none: a config predating scope must read unchanged", errs)
	}
}
