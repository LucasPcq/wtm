package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func scopeScanWith(services ...domain.ComposeService) map[string]domain.ComposeScan {
	return map[string]domain.ComposeScan{"docker-compose.yml": {File: "docker-compose.yml", Services: services}}
}

func choicesFor(t *testing.T, existing domain.RunConfig, services ...domain.ComposeService) []ServiceScopeChoice {
	t.Helper()
	return ServiceScopeChoices(ServiceScopeChoicesParams{
		Scans:    scopeScanWith(services...),
		Files:    []string{"docker-compose.yml"},
		Existing: existing,
	})
}

// A service built here serves this worktree's own source, so the question is
// not the reader's to answer. Shown all the same, with its reason.
func TestServiceScopeChoicesFixesABuiltService(t *testing.T) {
	got := choicesFor(t, domain.RunConfig{}, domain.ComposeService{Name: "api", HasBuild: true})
	if len(got) != 1 {
		t.Fatalf("choices = %v", got)
	}
	if !got[0].Fixed || got[0].Reason != domain.ScopeReasonBuild {
		t.Errorf("choice = %+v, want fixed with the build reason", got[0])
	}
	if got[0].Scope == domain.JobScopeShared {
		t.Error("a built service was proposed as shared")
	}
}

// A registry image is a genuinely open question, and nothing is pre-answered.
func TestServiceScopeChoicesLeavesAnImageOpen(t *testing.T) {
	got := choicesFor(t, domain.RunConfig{}, domain.ComposeService{Name: "db", Image: "postgres:16"})
	if got[0].Fixed {
		t.Error("an image service was pre-answered")
	}
	if got[0].Scope != domain.JobScopePerWorktree {
		t.Errorf("scope = %q, want the safe default", got[0].Scope)
	}
	if got[0].Tenant == nil || got[0].Tenant.Attach == "" {
		t.Errorf("tenant = %+v, want the postgres recipe pre-filled", got[0].Tenant)
	}
}

// The config outranks detection wherever it speaks: a re-init shows what was
// decided last time, not what a fresh look would propose.
func TestServiceScopeChoicesReadTheExistingConfigFirst(t *testing.T) {
	existing := domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "db", Scope: domain.JobScopeShared,
		Tenant: &domain.JobTenantConfig{Name: "mine_{worktree}", Attach: "my-script"},
	}}}

	got := choicesFor(t, existing, domain.ComposeService{Name: "db", Image: "postgres:16"})
	if got[0].Scope != domain.JobScopeShared {
		t.Errorf("scope = %q, want the config's own answer", got[0].Scope)
	}
	if got[0].Tenant == nil || got[0].Tenant.Attach != "my-script" {
		t.Errorf("tenant = %+v, want the one already written, not the recipe", got[0].Tenant)
	}
}

// A service the config declares per-worktree keeps that answer rather than
// being proposed for sharing all over again on every re-init.
func TestServiceScopeChoicesKeepAPerWorktreeAnswer(t *testing.T) {
	existing := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "db"}}}
	got := choicesFor(t, existing, domain.ComposeService{Name: "db", Image: "postgres:16"})
	if got[0].Scope != domain.JobScopePerWorktree || got[0].Tenant != nil {
		t.Errorf("choice = %+v, want the config's per-worktree answer with no recipe", got[0])
	}
}

// The list is complete, so a re-init cannot silently drop a service the reader
// had already answered for.
func TestServiceScopeChoicesListsEveryService(t *testing.T) {
	got := choicesFor(t, domain.RunConfig{},
		domain.ComposeService{Name: "db", Image: "postgres:16"},
		domain.ComposeService{Name: "api", HasBuild: true},
		domain.ComposeService{Name: "keycloak", Image: "quay.io/keycloak/keycloak:24"},
	)
	if len(got) != 3 {
		t.Fatalf("choices = %d, want every service listed", len(got))
	}
	if got[2].Tenant != nil {
		t.Errorf("keycloak tenant = %+v, want none: wtm knows no realm recipe", got[2].Tenant)
	}
}

func TestKnownTenantRecipeReadsThroughRegistryAndTag(t *testing.T) {
	for _, image := range []string{"postgres:16", "postgres", "docker.io/library/postgres:15-alpine"} {
		if got := KnownTenantRecipe(image); got == nil || got.Name != domain.TenantPostgresName {
			t.Errorf("KnownTenantRecipe(%q) = %+v, want the postgres recipe", image, got)
		}
	}
	if got := KnownTenantRecipe("redis:7"); got != nil {
		t.Errorf("KnownTenantRecipe(redis) = %+v, want none", got)
	}
}

func TestSharedFromChoicesKeepsOnlyWhatWasShared(t *testing.T) {
	got := SharedFromChoices([]ServiceScopeChoice{
		{File: "a.yml", Service: "db", Scope: domain.JobScopeShared},
		{File: "a.yml", Service: "api"},
		{File: "a.yml", Service: "built", Scope: domain.JobScopeShared, Fixed: true},
	})
	if len(got) != 1 || got[0].Service != "db" {
		t.Errorf("shared = %v, want db alone: a fixed choice is never shared", got)
	}
}
