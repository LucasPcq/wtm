package repository_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEachActionIsOfferedWithItsFollowUpOrRefusedWithWhy(t *testing.T) {
	offers := repository.Offers(repository.OffersParams{Command: "list", Branch: "main", Path: "/repo", Main: true})

	require.Len(t, offers, len(worktree.Actions()))
	for i, action := range worktree.Actions() {
		assert.Equal(t, action, offers[i].Action, "in worktree.Actions' order")
		assert.NotEqual(t, offers[i].FollowUp == nil, offers[i].Refused == nil, "%s is offered or refused, not both", action)
	}
	assert.Equal(t, worktree.UnofferedMainWorktree, *offerOf(t, offers, worktree.ActionClean).Refused)
	assert.Equal(t, &domain.FollowUp{
		Command: domain.CommandRunUp,
		Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{"main"}}},
		Code:    "list.run_up",
		Message: "Start the profile",
	}, offerOf(t, offers, worktree.ActionRunUp).FollowUp)
}

func TestAPullRequestKnownIsALinkAndAWorktreeOnNoBranchIsNamedByItsPath(t *testing.T) {
	linked := repository.Offers(repository.OffersParams{Command: "tree", Branch: "feat/a", Path: "/t/a", PRNumber: 12, PRURL: "https://github.com/acme/acme/pull/12"})
	detached := repository.Offers(repository.OffersParams{Command: "list", Path: "/t/x"})

	assert.Equal(t, &domain.FollowUp{URL: "https://github.com/acme/acme/pull/12", Code: "tree.open_pr", Message: "Open the pull request", Params: domain.Params{repository.ParamNumber: "12"}}, offerOf(t, linked, worktree.ActionOpenPR).FollowUp)
	assert.Equal(t, worktree.UnofferedNoBranch, *offerOf(t, detached, worktree.ActionOpenPR).Refused)
	assert.Equal(t, []string{"/t/x"}, offerOf(t, detached, worktree.ActionClean).FollowUp.Answers[0].Values)
	assert.Len(t, repository.Offered(detached), 3)
}
