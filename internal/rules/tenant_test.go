package rules_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestExpandTenantSubstitutesNameAndEnv(t *testing.T) {
	got, err := rules.ExpandTenant(rules.ExpandTenantParams{
		Tenant: domain.JobTenantConfig{
			Name: "crm_{worktree}",
			Env:  map[string]string{"CRM_DATABASE_URL": "postgres://h:5432/crm_{worktree}"},
		},
		Worktree: "feat_x",
		Ordinal:  2,
	})
	if err != nil {
		t.Fatalf("ExpandTenant: %v", err)
	}
	if got.Name != "crm_feat_x" {
		t.Errorf("name = %q, want crm_feat_x", got.Name)
	}
	if got.Env["CRM_DATABASE_URL"] != "postgres://h:5432/crm_feat_x" {
		t.Errorf("env = %q", got.Env["CRM_DATABASE_URL"])
	}
}

func TestExpandTenantSubstitutesOrdinal(t *testing.T) {
	got, err := rules.ExpandTenant(rules.ExpandTenantParams{
		Tenant:   domain.JobTenantConfig{Name: "slot{ordinal}"},
		Worktree: "feat_x",
		Ordinal:  3,
	})
	if err != nil {
		t.Fatalf("ExpandTenant: %v", err)
	}
	if got.Name != "slot3" {
		t.Errorf("name = %q, want slot3", got.Name)
	}
}

// An unknown placeholder is caught here rather than reaching a shell as literal
// braces, where it would silently create a database called "{branch}".
func TestExpandTenantRefusesUnknownToken(t *testing.T) {
	_, err := rules.ExpandTenant(rules.ExpandTenantParams{
		Tenant:   domain.JobTenantConfig{Name: "app_{branch}"},
		Worktree: "feat_x",
	})
	if !errors.Is(err, domain.ErrTenantUnknownToken) {
		t.Errorf("err = %v, want ErrTenantUnknownToken", err)
	}
}

func TestExpandTenantRefusesUnknownTokenInEnv(t *testing.T) {
	_, err := rules.ExpandTenant(rules.ExpandTenantParams{
		Tenant:   domain.JobTenantConfig{Name: "app", Env: map[string]string{"U": "postgres://h/{repo}"}},
		Worktree: "feat_x",
	})
	if !errors.Is(err, domain.ErrTenantUnknownToken) {
		t.Errorf("err = %v, want ErrTenantUnknownToken", err)
	}
}

func TestExpandTenantLeavesShellVariablesAlone(t *testing.T) {
	got, err := rules.ExpandTenant(rules.ExpandTenantParams{
		Tenant:   domain.JobTenantConfig{Name: "db", Env: map[string]string{"U": "postgres://h:$PORT/db"}},
		Worktree: "feat_x",
	})
	if err != nil {
		t.Fatalf("ExpandTenant: %v", err)
	}
	if got.Env["U"] != "postgres://h:$PORT/db" {
		t.Errorf("env = %q, shell variables must survive untouched", got.Env["U"])
	}
}

func TestTenantTokensAreTheShellVariables(t *testing.T) {
	got := rules.TenantTokens(rules.ExpandTenantParams{
		Tenant:   domain.JobTenantConfig{Name: "crm_{worktree}"},
		Worktree: "feat_x",
		Ordinal:  2,
	})
	if got[domain.EnvTenant] != "crm_feat_x" {
		t.Errorf("%s = %q", domain.EnvTenant, got[domain.EnvTenant])
	}
	if got[domain.EnvWorktree] != "feat_x" {
		t.Errorf("%s = %q", domain.EnvWorktree, got[domain.EnvWorktree])
	}
	if got[domain.EnvOrdinal] != "2" {
		t.Errorf("%s = %q", domain.EnvOrdinal, got[domain.EnvOrdinal])
	}
}

func TestIsSharedAndHasTenant(t *testing.T) {
	shared := domain.JobConfig{Scope: domain.JobScopeShared}
	if !rules.IsShared(shared) {
		t.Error("IsShared(shared) = false")
	}
	if rules.IsShared(domain.JobConfig{}) {
		t.Error("IsShared(default) = true, an absent scope is per-worktree")
	}
	if rules.HasTenant(shared) {
		t.Error("HasTenant with no block = true")
	}
	if rules.HasTenant(domain.JobConfig{Tenant: &domain.JobTenantConfig{Name: "n"}}) {
		t.Error("HasTenant with no attach = true")
	}
	if !rules.HasTenant(domain.JobConfig{Tenant: &domain.JobTenantConfig{Name: "n", Attach: "true"}}) {
		t.Error("HasTenant with name and attach = false")
	}
}
