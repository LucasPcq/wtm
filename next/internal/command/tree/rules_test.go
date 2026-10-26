package tree_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestANodeNeedsTheFirstActionThatApplies(t *testing.T) {
	behindOrigin := &worktree.Origin{Behind: 2, State: worktree.Behind}
	cases := map[string]struct {
		detail  tree.Detail
		cleans  bool
		command string
		code    string
	}{
		"rebasing_first":     {detail: tree.Detail{Branch: "feat/a", Condition: worktree.ConditionRebasing, NeedsSync: true}, cleans: true, command: "go", code: "tree.go"},
		"merged_before_sync": {detail: tree.Detail{Branch: "feat/a", NeedsSync: true}, cleans: true, command: "prune", code: "tree.prune"},
		"needs_sync":         {detail: tree.Detail{Branch: "feat/a", NeedsSync: true}, command: "sync", code: "tree.sync"},
		"merged_before_ff":   {detail: tree.Detail{Branch: "feat/a", Origin: behindOrigin}, cleans: true, command: "prune", code: "tree.prune"},
		"ff_before_sync":     {detail: tree.Detail{Branch: "feat/a", NeedsSync: true, Origin: behindOrigin}, command: "fast-forward", code: "tree.fast-forward"},
		"diverged_not_ff":    {detail: tree.Detail{Branch: "feat/a", Origin: &worktree.Origin{Ahead: 1, Behind: 2, State: worktree.Diverged}}},
		"missing_not_moved":  {detail: tree.Detail{Branch: "feat/a", Missing: true, Condition: worktree.ConditionMissing, NeedsSync: true, Origin: behindOrigin}},
		"busy_not_ff":        {detail: tree.Detail{Branch: "feat/a", Condition: worktree.ConditionMerging, Origin: behindOrigin}, command: "go", code: "tree.go"},
		"nothing_due":        {detail: tree.Detail{Branch: "feat/a"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			actions := tree.Actions(c.detail, c.cleans)

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

func TestANodeGitStoppedInTheMiddleOfAnOperationIsSentToFinishIt(t *testing.T) {
	for _, operation := range worktree.Operations() {
		actions := tree.Actions(tree.Detail{Branch: "feat/a", Condition: worktree.Condition(operation), NeedsSync: true}, true)

		require.Len(t, actions, 1, operation)
		assert.Equal(t, "go", actions[0].Command, operation)
		assert.NotRegexp(t, `its $`, actions[0].Message, operation)
	}
}
