package teardown_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
	"github.com/stretchr/testify/assert"
)

func TestAChildMovesPastEveryParentRemovedOntoTheNearestThatStays(t *testing.T) {
	facts := teardown.Facts{
		Repository: worktree.Repository{
			Config: project.Config{Worktrees: project.Worktrees{BaseBranch: "main"}},
			Snapshot: worktree.Snapshot{Worktrees: []worktree.Worktree{
				{Path: "/main", Main: true, State: worktree.OnBranch{Branch: "main"}},
				{Path: "/a", State: worktree.OnBranch{Branch: "a"}},
				{Path: "/b", State: worktree.OnBranch{Branch: "b"}},
				{Path: "/c", State: worktree.OnBranch{Branch: "c"}},
			}},
		},
		Records: map[string]worktree.Meta{"a": {SourceBranch: "gone"}, "b": {SourceBranch: "a"}, "c": {SourceBranch: "b"}},
	}

	onGrandparent := facts.Moves([]teardown.Removal{{Worktree: "b", Branch: "b"}})
	pastAll := facts.Moves([]teardown.Removal{{Worktree: "a", Branch: "a"}, {Worktree: "b", Branch: "b"}})

	assert.Equal(t, []teardown.Move{{Branch: "c", From: "b", To: "a"}}, onGrandparent)
	assert.Equal(t, []teardown.Move{{Branch: "c", From: "b", To: "main", Base: true}}, pastAll, "a's own parent no longer exists: the base branch ends the chain")
	assert.Equal(t, "/c", facts.ChildPath(onGrandparent[0]))
}

func TestAChildLeftBehindIsWarnedOfWithTheReparentThatMovesIt(t *testing.T) {
	warnings := teardown.OrphanedBy(command, []teardown.Move{{Branch: "c", From: "b", To: "a"}})

	assert.Equal(t, []string{"sweep.orphaned"}, codes(warnings))
	assert.Equal(t, "sweep.reparent", warnings[0].FollowUp.Code)
	assert.Equal(t, "reparent", warnings[0].FollowUp.Command)
}
