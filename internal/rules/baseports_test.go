package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// A job whose command ignores the port it was given binds the base port in
// whichever worktree it runs, so the neighbour to name is not necessarily the
// main checkout — which is exactly the case a field test reproduced.
func TestBasePortOwnersNamesAnyNeighbourRunningTheJob(t *testing.T) {
	got := rules.BasePortOwners(rules.BasePortOwnersParams{
		SelfWorkDir: "/trees/b",
		Jobs: []domain.JobConfig{
			{Name: "web", Ports: map[string]int{"PORT": 3000}},
			{Name: "api", Ports: map[string]int{"PORT": 4000}},
		},
		Running: []domain.JobInfo{
			{Name: "web", WorkDir: "/trees/a", Status: domain.JobStatusRunning},
			{Name: "api", WorkDir: "/trees/b", Status: domain.JobStatusRunning},
		},
		Holders: []rules.PortHolder{
			{WorkDir: "/trees/a", Worktree: "feat/a"},
			{WorkDir: "/trees/b", Worktree: "feat/b"},
		},
	})

	if got[3000] != "feat/a" {
		t.Errorf("owner of 3000 = %q, want feat/a", got[3000])
	}
	if _, held := got[4000]; held {
		t.Errorf("4000 is this worktree's own job: %v", got)
	}
}

func TestBasePortOwnersIgnoresStoppedJobs(t *testing.T) {
	got := rules.BasePortOwners(rules.BasePortOwnersParams{
		SelfWorkDir: "/trees/b",
		Jobs:        []domain.JobConfig{{Name: "web", Ports: map[string]int{"PORT": 3000}}},
		Running:     []domain.JobInfo{{Name: "web", WorkDir: "/trees/a", Status: domain.JobStatusStopped}},
		Holders:     []rules.PortHolder{{WorkDir: "/trees/a", Worktree: "feat/a"}},
	})

	if len(got) != 0 {
		t.Errorf("got %v, want no owner", got)
	}
}

// A worktree git cannot name is left out rather than named by its path: the
// message exists to be recognised.
func TestBasePortOwnersSkipsAnUnnamedWorktree(t *testing.T) {
	got := rules.BasePortOwners(rules.BasePortOwnersParams{
		SelfWorkDir: "/trees/b",
		Jobs:        []domain.JobConfig{{Name: "web", Ports: map[string]int{"PORT": 3000}}},
		Running:     []domain.JobInfo{{Name: "web", WorkDir: "/trees/a", Status: domain.JobStatusRunning}},
	})

	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}
