package domain

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
)

func TestTheSourceOptionsAreTheLocalBranchesThenThoseOnlyOnOriginTheExceptedLeftOut(t *testing.T) {
	snapshot := worktree.Snapshot{
		Branches:   worktree.Branches{Local: []worktree.Branch{{Name: "main"}, {Name: "feat/x"}}, Remote: []worktree.Branch{{Name: "main"}, {Name: "feat/y"}, {Name: "feat/z"}}},
		Divergence: map[string]worktree.AheadBehind{"main": {Behind: 2}},
	}

	repo := worktree.Repository{Config: project.Config{Worktrees: project.Worktrees{BaseBranch: "main"}}, Snapshot: snapshot}

	assert.Equal(t, []Option{{Value: "main", Label: "main (default)", Badge: "2 behind"}, {Value: "origin/feat/y"}}, SourceOptions(repo, "feat/x", "feat/z"))
}
