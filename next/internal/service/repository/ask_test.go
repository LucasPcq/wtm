package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"

	"github.com/stretchr/testify/assert"
)

func TestAReadAsksTheDaemonAndTheForgeOfItsBranchesBesideReadingTheWorktrees(t *testing.T) {
	var asked []string
	read := false
	params := repository.AskParams{
		Jobs: func(context.Context) ([]run.Job, error) { return []run.Job{{Name: "web"}}, nil },
		PullRequests: func(_ context.Context, branches []string) ([]worktree.PullRequest, error) {
			asked = branches
			return []worktree.PullRequest{{Branch: "feat/a", Number: 1}}, errors.New("rate limited")
		},
		Branches: []string{"feat/a"},
		WithPRs:  true,
	}

	got := repository.Ask(t.Context(), params, func() { read = true })

	assert.True(t, read)
	assert.Equal(t, []string{"feat/a"}, asked)
	assert.Equal(t, []run.Job{{Name: "web"}}, got.Jobs)
	assert.Len(t, got.PullRequests, 1, "what the forge found before failing is kept")
	assert.EqualError(t, got.PRsErr, "rate limited")
	assert.False(t, got.Interrupted)
}

func TestTheForgeIsAskedOnlyWithPRs(t *testing.T) {
	params := repository.AskParams{
		Jobs:         func(context.Context) ([]run.Job, error) { return nil, nil },
		PullRequests: func(context.Context, []string) ([]worktree.PullRequest, error) { panic("the forge was asked") },
	}

	got := repository.Ask(t.Context(), params, func() {})

	assert.Nil(t, got.PullRequests)
	assert.False(t, got.Interrupted)
}

func TestAReadWhoseContextIsDoneByItsEndIsInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	params := repository.AskParams{Jobs: func(context.Context) ([]run.Job, error) { return nil, nil }}

	got := repository.Ask(ctx, params, cancel)

	assert.True(t, got.Interrupted)
}

func TestWhatAReadCouldNotAskIsAWarningUnderItsCommandsCodes(t *testing.T) {
	asked := repository.Asked{JobsErr: run.ErrUnknown, PRsErr: errors.New("gh is not logged in")}

	warnings := asked.Warnings("list")

	assert.Equal(t, []domain.Warning{
		{Code: "list.jobs_unknown", Message: run.ErrUnknown.Error()},
		{Code: "list.prs_unavailable", Message: "gh is not logged in"},
	}, warnings)
}

func TestAnInterruptedReadWarnsOfNothingItCouldNotAsk(t *testing.T) {
	asked := repository.Asked{JobsErr: run.ErrUnknown, PRsErr: errors.New("context canceled"), Interrupted: true}

	assert.Empty(t, asked.Warnings("list"))
}

func TestTheServicesOfAWorktreeAreUnknownWhenTheDaemonCannotTell(t *testing.T) {
	jobs := []run.Job{{Name: "web", WorkDir: "/a", Up: true}}

	assert.Equal(t, []string{"web"}, repository.Asked{Jobs: jobs}.Services("/a"))
	assert.Nil(t, repository.Asked{Jobs: jobs, JobsErr: run.ErrUnknown}.Services("/a"))
}
