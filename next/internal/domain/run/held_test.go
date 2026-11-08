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
	assert.Equal(t, 4, Count(got))
	assert.Equal(t, []HeldRepo{}, Held(nil))
}
