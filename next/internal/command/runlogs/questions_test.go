package runlogs_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEachAnswerOfTheFormComesBackAsItsValues(t *testing.T) {
	req := runlogs.Request{Worktrees: []string{featA, featB}, Job: "api", Follow: true}

	folded, err := domain.Fold(runlogs.Request{}, domain.AnswersOf(runlogs.Questions{}.Form(), req))

	require.NoError(t, err)
	assert.Equal(t, req, folded)
}

func TestTheQuestionsOfferEveryStandingWorktreeAndEveryJob(t *testing.T) {
	w := newWorld(t)

	questions, err := w.usecase.Questions(t.Context(), runlogs.Request{Caller: domain.Caller{Cwd: w.at(featA)}})

	require.NoError(t, err)
	assert.Equal(t, []string{featA}, questions.Worktrees.Picked, "the one the caller stands in, picked")
	assert.Equal(t, domain.OriginDefault, questions.Worktrees.Origin)
	var worktrees, jobs []string
	for _, option := range questions.Worktrees.Options {
		worktrees = append(worktrees, option.Value)
	}
	for _, option := range questions.Jobs {
		jobs = append(jobs, option.Value)
	}
	assert.Equal(t, []string{"main", featA, featB}, worktrees, "a worktree whose directory is gone is not offered")
	assert.Equal(t, []string{"api", "web", "migrate", "crashy", "db", "never", "stack"}, jobs)
	assert.Empty(t, questions.Errors)
}
