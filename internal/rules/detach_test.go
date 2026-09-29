package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func sharedWithNamespace(name string) domain.JobConfig {
	return domain.JobConfig{
		Name: name, Kind: domain.JobKindService, Scope: domain.JobScopeShared,
		Namespace: &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "true", Remove: "true"},
	}
}

// A job with nothing to give back has nothing to ask of a removal.
func TestRemovableKeepsOnlySharedJobsWithARemoveCommand(t *testing.T) {
	noRemove := sharedWithNamespace("cache")
	noRemove.Namespace.Remove = ""
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		sharedWithNamespace("postgres"),
		noRemove,
		{Name: "api", Kind: domain.JobKindService},
	}}

	got := Removable(cfg).Jobs
	if len(got) != 1 || got[0].Name != "postgres" {
		t.Errorf("removable = %+v, want postgres alone", got)
	}
}

func TestHeldNamespacesNamesEachSliceFromTheWorktreesEnv(t *testing.T) {
	holdings := []domain.NamespaceHolding{{
		Branch: "feat/x",
		Env:    map[string]string{domain.EnvWorktree: "feat-x"},
		Config: domain.RunConfig{Jobs: []domain.JobConfig{sharedWithNamespace("postgres")}},
	}}

	held := HeldNamespaces(HeldNamespacesParams{Holdings: holdings, Up: map[string]bool{"postgres": true}})
	want := []domain.HeldNamespace{{Name: "app_feat-x", Job: "postgres", Up: true}}
	if !slices.Equal(held, want) {
		t.Errorf("held = %+v, want %+v", held, want)
	}
}

// Two worktrees holding data in the same service down are one service to start.
func TestDownServicesNamesEachServiceOnce(t *testing.T) {
	held := []domain.HeldNamespace{
		{Name: "app_a", Job: "postgres"},
		{Name: "app_b", Job: "postgres"},
		{Name: "realm_a", Job: "keycloak", Up: true},
		{Name: "cache_a", Job: "redis"},
	}
	if got := DownServices(held); !slices.Equal(got, []string{"postgres", "redis"}) {
		t.Errorf("down = %v, want postgres, redis", got)
	}
}

// The recap says what happens to each namespace, per the answer: a service up
// drops it, a service down either starts or keeps it.
func TestDataRecapLinesFollowTheAnswer(t *testing.T) {
	held := []domain.HeldNamespace{{Name: "app_a", Job: "postgres", Up: true}, {Name: "realm_a", Job: "keycloak"}}

	started := strings.Join(DataRecapLines(DataRecapLinesParams{Held: held, StartDown: true}), "\n")
	if !strings.Contains(started, "starts keycloak") {
		t.Errorf("recap does not say keycloak is started:\n%s", started)
	}
	deferred := strings.Join(DataRecapLines(DataRecapLinesParams{Held: held}), "\n")
	if !strings.Contains(deferred, "realm_a kept until keycloak next starts") {
		t.Errorf("recap does not say realm_a is kept:\n%s", deferred)
	}
	if !strings.Contains(deferred, "app_a in postgres") {
		t.Errorf("recap lost the namespace dropped from a service up:\n%s", deferred)
	}
}

func TestDataRecapLinesSayOnceThatTheDataIsKept(t *testing.T) {
	held := []domain.HeldNamespace{{Name: "app_a", Job: "postgres"}, {Name: "app_b", Job: "postgres"}}
	got := DataRecapLines(DataRecapLinesParams{Held: held, KeepData: true})
	if !slices.Equal(got, []string{domain.CleanKeepDataLine}) {
		t.Errorf("lines = %v, want the keep-data line alone", got)
	}
}

func TestDataRecapLinesEmptyWhenNothingIsHeld(t *testing.T) {
	if got := DataRecapLines(DataRecapLinesParams{KeepData: true}); len(got) != 0 {
		t.Errorf("lines = %v, want none", got)
	}
}
