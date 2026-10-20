package tree_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/stretchr/testify/assert"
)

func TestANodeNeedsTheFirstActionThatApplies(t *testing.T) {
	merged := &tree.PR{Number: 12, State: worktree.PullRequestMerged}
	cases := map[string]struct {
		detail  tree.Detail
		command string
		code    string
	}{
		"rebasing_first":       {detail: tree.Detail{Branch: "feat/a", Rebasing: true, PR: merged, NeedsSync: true}, command: "go", code: "tree.go"},
		"merged_before_sync":   {detail: tree.Detail{Branch: "feat/a", PR: merged, NeedsSync: true}, command: "clean", code: "tree.clean"},
		"needs_sync":           {detail: tree.Detail{Branch: "feat/a", NeedsSync: true, PR: &tree.PR{State: worktree.PullRequestOpen}}, command: "sync", code: "tree.sync"},
		"closed_not_cleaned":   {detail: tree.Detail{Branch: "feat/a", PR: &tree.PR{State: worktree.PullRequestClosed}}},
		"main_merged_not_gone": {detail: tree.Detail{Branch: "main", Main: true, PR: merged}},
		"base_merged_not_gone": {detail: tree.Detail{Branch: "develop", PR: merged}},
		"nothing_due":          {detail: tree.Detail{Branch: "feat/a"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			actions := tree.Actions(c.detail, "develop")

			if c.command == "" {
				assert.Equal(t, []domain.FollowUp{}, actions)
				return
			}
			assert.Len(t, actions, 1)
			assert.Equal(t, c.command, actions[0].Command)
			assert.Equal(t, c.code, actions[0].Code)
			assert.Equal(t, []domain.Answered{{Path: domain.PathWorktree, Values: []string{c.detail.Branch}}}, actions[0].Answers)
		})
	}
}
