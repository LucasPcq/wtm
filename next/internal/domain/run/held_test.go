package run

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJobsAreHeldByRepositoryEachWorktreesTogether(t *testing.T) {
	jobs := []Job{
		{Name: "web", Repo: "/b", Branch: "main", WorkDir: "/b", Hold: HoldOwn},
		{Name: "api", Repo: "/a", Branch: "x", WorkDir: "/a/.trees/x", Hold: HoldOwn},
		{Name: "db", Repo: "/a", Branch: "main", WorkDir: "/a", Hold: HoldOwn, SharedDir: "/a"},
		{Name: "db", Repo: "/a", Branch: "x", WorkDir: "/a/.trees/x", Hold: HoldClaim, SharedDir: "/a"},
	}

	got := Held(jobs)

	assert.Equal(t, []HeldRepo{
		{Root: "/a", Worktrees: []HeldWorktree{
			{Branch: "main", Path: "/a", Jobs: []HeldJob{{Name: "db", Hold: HoldOwn, Shared: true}}},
			{Branch: "x", Path: "/a/.trees/x", Jobs: []HeldJob{{Name: "api", Hold: HoldOwn}, {Name: "db", Hold: HoldClaim}}},
		}},
		{Root: "/b", Worktrees: []HeldWorktree{{Branch: "main", Path: "/b", Jobs: []HeldJob{{Name: "web", Hold: HoldOwn}}}}},
	}, got)
	assert.Equal(t, []HeldRepo{}, Held(nil))
}

func TestAJobHeldIsNamedWithItsWorktreeAndItsRepository(t *testing.T) {
	repos := Held([]Job{{Name: "web", Repo: "/src/acme", Branch: "feat/a", WorkDir: "/src/acme.trees/feat-a"}, {Name: "docs", Repo: "/src/wtm", WorkDir: "/src/wtm.trees/x"}, {Name: "idle", WorkDir: "/tmp/x"}})

	assert.Equal(t, []string{"idle (/tmp/x)", "web (feat/a · acme)", "docs (/src/wtm.trees/x · wtm)"}, Names(repos))
}

func TestAClaimRecordedForNoRepositoryIsPlacedBesideItsWorktreesJobs(t *testing.T) {
	jobs := []Job{
		{Name: "api", Repo: "/a", Branch: "x", WorkDir: "/a/.trees/x", Hold: HoldOwn},
		{Name: "db", WorkDir: "/a/.trees/x", Hold: HoldClaim, SharedDir: "/a"},
		{Name: "db", WorkDir: "/a/.trees/y", Hold: HoldClaim, SharedDir: "/a"},
		{Name: "db", Repo: "/a", Branch: "main", WorkDir: "/a", Hold: HoldOwn, SharedDir: "/a"},
	}

	assert.Equal(t, []HeldRepo{{Root: "/a", Worktrees: []HeldWorktree{
		{Branch: "main", Path: "/a", Jobs: []HeldJob{{Name: "db", Hold: HoldOwn, Shared: true}}},
		{Branch: "x", Path: "/a/.trees/x", Jobs: []HeldJob{{Name: "api", Hold: HoldOwn}, {Name: "db", Hold: HoldClaim}}},
		{Path: "/a/.trees/y", Jobs: []HeldJob{{Name: "db", Hold: HoldClaim}}},
	}}}, Held(jobs))
}
