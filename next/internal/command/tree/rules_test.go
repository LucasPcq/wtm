package tree_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestANodeOffersEveryActionOfItsWorktreeAnsweredByItsBranch(t *testing.T) {
	path := "/repo/.trees/feat/a"
	actions := tree.Actions(tree.ActionsParams{Detail: tree.Detail{Branch: "feat/a", Path: &path}})

	require.Len(t, actions, len(worktree.Actions()))
	sync := offerOf(t, actions, worktree.ActionSync)
	require.NotNil(t, sync.FollowUp)
	assert.Equal(t, "tree.sync", sync.FollowUp.Code)
	assert.Equal(t, []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/a"}}}, sync.FollowUp.Answers)
	assert.Equal(t, worktree.UnofferedNoPullRequest, *offerOf(t, actions, worktree.ActionOpenPR).Refused)
}

func TestTheBaseBranchsNodeIsNotRemoved(t *testing.T) {
	path := "/repo/.trees/develop"

	clean := offerOf(t, tree.Actions(tree.ActionsParams{Detail: tree.Detail{Branch: "develop", Path: &path}, Base: true}), worktree.ActionClean)

	assert.Nil(t, clean.FollowUp)
	assert.Equal(t, worktree.UnofferedBaseBranch, *clean.Refused)
}
