package resolve_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/resolve"
	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheRequestReadsBackItsAnswersButNotWhereItStands(t *testing.T) {
	req := resolve.Request{Worktree: "feat/a", Caller: domain.Caller{Cwd: mainPath}}

	folded, err := domain.Fold(resolve.Request{}, domain.AnswersOf(resolve.Questions{}.Form(), req))

	require.NoError(t, err)
	assert.Equal(t, resolve.Request{Worktree: "feat/a"}, folded)
}

func TestThePickerOffersEveryWorktreeTheCursorOnTheOneYouAreIn(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: several}})

	q, err := w.usecase.Questions(t.Context(), resolve.Request{Caller: domain.Caller{Cwd: "/repo/.trees/fix/nav/src"}})

	require.NoError(t, err)
	field := selectOf(t, q)
	assert.Equal(t, "fix/nav", field.Value)
	assert.Equal(t, []domain.Option{
		{Value: "main", Tag: domain.TagMain},
		{Value: "/repo/.trees/detached", Detached: true},
		{Value: "feat/login"},
		{Value: "feat/logout"},
		{Value: "fix/nav", Tag: domain.TagCurrent},
	}, field.Options, "a worktree on no branch is offered by its path")
	assert.Empty(t, q.Form().Problems)
}

func TestAnAmbiguousNameOffersOnlyTheWorktreesItMatches(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: several}})

	q, err := w.usecase.Questions(t.Context(), resolve.Request{Worktree: "log"})

	require.NoError(t, err)
	field := selectOf(t, q)
	assert.Empty(t, field.Value)
	assert.Equal(t, []domain.Option{{Value: "feat/login"}, {Value: "feat/logout"}}, field.Options)
	require.Len(t, q.Form().Problems, 1)
	assert.Equal(t, domain.CodeAmbiguous, q.Form().Problems[0].Code)
}
