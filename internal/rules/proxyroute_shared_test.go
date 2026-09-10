package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func publishedJob(scope domain.JobScope) domain.JobConfig {
	return domain.JobConfig{
		Name:  "db",
		Scope: scope,
		Ports: map[string]int{"DB_PORT": 5432},
		URL:   &domain.JobURLConfig{Port: "DB_PORT"},
	}
}

// One instance cannot answer under two names. Keeping the worktree segment
// would publish db.feat-a.projet.localhost and db.feat-b.projet.localhost onto
// the same target: two names for one thing.
func TestRouteHostDropsTheWorktreeForASharedJob(t *testing.T) {
	got := RouteHost(RouteHostParams{
		Job: publishedJob(domain.JobScopeShared), Worktree: "feat-a", Project: "projet",
	})
	if got != "db.projet."+domain.ProxyTLD {
		t.Errorf("host = %q, want db.projet.%s", got, domain.ProxyTLD)
	}
}

func TestRouteHostKeepsTheWorktreeForAPerWorktreeJob(t *testing.T) {
	got := RouteHost(RouteHostParams{
		Job: publishedJob(domain.JobScopePerWorktree), Worktree: "feat-a", Project: "projet",
	})
	if got != "db.feat-a.projet."+domain.ProxyTLD {
		t.Errorf("host = %q, want db.feat-a.projet.%s", got, domain.ProxyTLD)
	}
}

// Two worktrees must resolve a shared job to the very same host, or the two
// .env files would disagree about where one service answers.
func TestRouteHostIsStableAcrossWorktreesForASharedJob(t *testing.T) {
	job := publishedJob(domain.JobScopeShared)
	first := RouteHost(RouteHostParams{Job: job, Worktree: "feat-a", Project: "projet"})
	second := RouteHost(RouteHostParams{Job: job, Worktree: "feat-b", Project: "projet"})
	if first != second {
		t.Errorf("hosts differ: %q vs %q", first, second)
	}
}
