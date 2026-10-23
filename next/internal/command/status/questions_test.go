package status_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheRequestReadsBackItsAnswersButNotWhereItStands(t *testing.T) {
	req := status.Request{Worktree: "feat/a", WithPRs: true, Caller: domain.Caller{Cwd: "/repo/main"}}

	folded, err := domain.Fold(status.Request{}, domain.AnswersOf(status.Questions{}.Form(), req))

	require.NoError(t, err)
	assert.Equal(t, status.Request{Worktree: "feat/a", WithPRs: true}, folded)
}

func TestThePickerOffersEveryWorktreeOnTheOneTheRequestReads(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a"), {Path: "/repo/.trees/detached", State: worktree.Detached{}}}}})

	q, err := w.usecase.Questions(t.Context(), status.Request{Caller: domain.Caller{Cwd: "/repo/.trees/feat/a"}})

	require.NoError(t, err)
	field, ok := q.Form().Fields[0].(domain.SelectField)
	require.True(t, ok)
	assert.Equal(t, "feat/a", field.Value)
	assert.Equal(t, []domain.Option{
		{Value: "main", Badge: "main"},
		{Value: "/repo/.trees/detached", Label: "/repo/.trees/detached (detached)"},
		{Value: "feat/a", Badge: "current"},
	}, field.Options, "a worktree on no branch is offered and says so, rather than refused once picked")
	assert.Empty(t, field.Skip)
}

func TestReadingEveryWorktreeAsksNoneAndANameNoWorktreeAnswersIsAProblem(t *testing.T) {
	w := newWorld(seed{})

	all, err := w.usecase.Questions(t.Context(), status.Request{All: true})
	require.NoError(t, err)
	unknown, err := w.usecase.Questions(t.Context(), status.Request{Worktree: "nope"})
	require.NoError(t, err)

	assert.NotEmpty(t, all.Form().Fields[0].Spec().Skip)
	assert.Equal(t, []string{domain.CodeNotFound}, fieldCodes(unknown.Form().Problems))
}
