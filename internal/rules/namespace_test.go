package rules_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestExpandNamespaceSubstitutesNameAndEnv(t *testing.T) {
	got, err := rules.ExpandNamespace(rules.ExpandNamespaceParams{
		Namespace: domain.JobNamespaceConfig{
			Name: "crm_{worktree}",
			Env:  map[string]string{"CRM_DATABASE_URL": "postgres://h:5432/crm_{worktree}"},
		},
		Worktree: "feat_x",
		Ordinal:  2,
	})
	if err != nil {
		t.Fatalf("ExpandNamespace: %v", err)
	}
	if got.Name != "crm_feat_x" {
		t.Errorf("name = %q, want crm_feat_x", got.Name)
	}
	if got.Env["CRM_DATABASE_URL"] != "postgres://h:5432/crm_feat_x" {
		t.Errorf("env = %q", got.Env["CRM_DATABASE_URL"])
	}
}

func TestExpandNamespaceSubstitutesOrdinal(t *testing.T) {
	got, err := rules.ExpandNamespace(rules.ExpandNamespaceParams{
		Namespace: domain.JobNamespaceConfig{Name: "slot{ordinal}"},
		Worktree:  "feat_x",
		Ordinal:   3,
	})
	if err != nil {
		t.Fatalf("ExpandNamespace: %v", err)
	}
	if got.Name != "slot3" {
		t.Errorf("name = %q, want slot3", got.Name)
	}
}

// An unknown placeholder is caught here rather than reaching a shell as literal
// braces, where it would silently create a database called "{branch}".
func TestExpandNamespaceRefusesUnknownToken(t *testing.T) {
	_, err := rules.ExpandNamespace(rules.ExpandNamespaceParams{
		Namespace: domain.JobNamespaceConfig{Name: "app_{branch}"},
		Worktree:  "feat_x",
	})
	if !errors.Is(err, domain.ErrNamespaceUnknownToken) {
		t.Errorf("err = %v, want ErrNamespaceUnknownToken", err)
	}
}

func TestExpandNamespaceRefusesUnknownTokenInEnv(t *testing.T) {
	_, err := rules.ExpandNamespace(rules.ExpandNamespaceParams{
		Namespace: domain.JobNamespaceConfig{Name: "app", Env: map[string]string{"U": "postgres://h/{repo}"}},
		Worktree:  "feat_x",
	})
	if !errors.Is(err, domain.ErrNamespaceUnknownToken) {
		t.Errorf("err = %v, want ErrNamespaceUnknownToken", err)
	}
}

func TestExpandNamespaceLeavesShellVariablesAlone(t *testing.T) {
	got, err := rules.ExpandNamespace(rules.ExpandNamespaceParams{
		Namespace: domain.JobNamespaceConfig{Name: "db", Env: map[string]string{"U": "postgres://h:$PORT/db"}},
		Worktree:  "feat_x",
	})
	if err != nil {
		t.Fatalf("ExpandNamespace: %v", err)
	}
	if got.Env["U"] != "postgres://h:$PORT/db" {
		t.Errorf("env = %q, shell variables must survive untouched", got.Env["U"])
	}
}

func TestNamespaceTokensAreTheShellVariables(t *testing.T) {
	got := rules.NamespaceTokens(rules.ExpandNamespaceParams{
		Namespace: domain.JobNamespaceConfig{Name: "crm_{worktree}"},
		Worktree:  "feat_x",
		Ordinal:   2,
	})
	if got[domain.EnvNamespace] != "crm_feat_x" {
		t.Errorf("%s = %q", domain.EnvNamespace, got[domain.EnvNamespace])
	}
	if got[domain.EnvWorktree] != "feat_x" {
		t.Errorf("%s = %q", domain.EnvWorktree, got[domain.EnvWorktree])
	}
	if got[domain.EnvOrdinal] != "2" {
		t.Errorf("%s = %q", domain.EnvOrdinal, got[domain.EnvOrdinal])
	}
}

func TestIsSharedAndHasNamespace(t *testing.T) {
	shared := domain.JobConfig{Scope: domain.JobScopeShared}
	if !rules.IsShared(shared) {
		t.Error("IsShared(shared) = false")
	}
	if rules.IsShared(domain.JobConfig{}) {
		t.Error("IsShared(default) = true, an absent scope is per-worktree")
	}
	if rules.HasNamespace(shared) {
		t.Error("HasNamespace with no block = true")
	}
	if rules.HasNamespace(domain.JobConfig{Namespace: &domain.JobNamespaceConfig{Name: "n"}}) {
		t.Error("HasNamespace with no attach = true")
	}
	if !rules.HasNamespace(domain.JobConfig{Namespace: &domain.JobNamespaceConfig{Name: "n", Create: "true"}}) {
		t.Error("HasNamespace with name and attach = false")
	}
}

// A worktree created and thrown away without ever starting the stack holds no
// namespace, so a clean owes nothing — running its detach would be a DROP DATABASE
// on a database that never existed.
func TestNamespaceJobsStartedOnlyRemembersWhatCameUp(t *testing.T) {
	jobs := []domain.JobConfig{
		{Name: "db", Scope: domain.JobScopeShared, Namespace: &domain.JobNamespaceConfig{Name: "n", Create: "true"}},
		{Name: "cache", Scope: domain.JobScopeShared},
		{Name: "web"},
	}

	if got := rules.NamespaceJobsStarted(rules.NamespaceJobsStartedParams{Jobs: jobs}); len(got) != 0 {
		t.Errorf("nothing started, got %v", got)
	}

	got := rules.NamespaceJobsStarted(rules.NamespaceJobsStartedParams{Jobs: jobs, Started: []string{"db", "cache", "web"}})
	if len(got) != 1 || got[0] != "db" {
		t.Errorf("got %v, want db alone: cache carves nothing out and web is not shared", got)
	}
}

func TestJobsHeldNarrowsToTheRecordedSharedJobs(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "db", Scope: domain.JobScopeShared},
		{Name: "cache", Scope: domain.JobScopeShared},
		{Name: "web"},
	}}

	got := rules.JobsHeld(cfg, []string{"db", "web"})
	if len(got.Jobs) != 1 || got.Jobs[0].Name != "db" {
		t.Errorf("jobs = %v, want db alone: web is not shared", got.Jobs)
	}
	if len(rules.JobsHeld(cfg, nil).Jobs) != 0 {
		t.Error("an empty record must narrow to nothing")
	}
}
