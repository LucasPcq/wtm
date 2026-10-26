package fastforward_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThePickerOffersEveryWorktreeOnABranchWithWhereItStandsAsLastFetched(t *testing.T) {
	w := newWorld(t)

	q := w.questions(t, fastforward.Request{})

	options := q.Worktrees.Options
	assert.Equal(t, []string{"main", ahead, behind, bisecting, dirty, dirtyLatest, diverged, gone, latest, picking, rebasing, stale, unpushed}, valuesOf(options))
	assert.Equal(t, domain.TagMain, options[0].Tag)
	assert.Equal(t, &worktree.Origin{Ahead: 1, State: worktree.Ahead}, optionOf(options, ahead).Origin)
	assert.Nil(t, optionOf(options, unpushed).Origin)
	assert.Equal(t, 1, optionOf(options, dirty).Changes)
}

func TestThePickerPicksTheWorktreesBehindOriginByDefault(t *testing.T) {
	w := newWorld(t)

	worktrees, ok := fieldOf(w.questions(t, fastforward.Request{}).Form(), domain.PathWorktree).(domain.MultiSelectField)

	require.True(t, ok)
	assert.Equal(t, []string{"main", behind}, worktrees.Picked)
	assert.Equal(t, domain.OriginDefault, worktrees.Origin)
}

func TestTheDefaultPickIsWhatARequestNamingNoneMoves(t *testing.T) {
	w := newWorld(t)
	worktrees, ok := fieldOf(w.questions(t, fastforward.Request{}).Form(), domain.PathWorktree).(domain.MultiSelectField)
	require.True(t, ok)

	planned, _ := w.plan(t, fastforward.Request{})

	assert.Equal(t, append(worktrees.Picked, stale), moving(planned), "stale is behind only once origin is fetched")
}

func TestForceIsAskedOnlyWhenAWorktreeBehindOriginHasUncommittedChanges(t *testing.T) {
	w := newWorld(t)

	assert.Equal(t, domain.SkipSafe, fieldOf(w.questions(t, naming(behind, dirtyLatest)).Form(), fastforward.PathForce).Spec().Skip)
	assert.Empty(t, fieldOf(w.questions(t, naming(behind, dirty)).Form(), fastforward.PathForce).Spec().Skip)
	assert.Equal(t, []string{dirty}, []string{w.questions(t, naming(behind, dirty)).Unsafe[0].Worktree})
	assert.Equal(t, domain.SkipSafe, fieldOf(w.questions(t, fastforward.Request{All: true}).Form(), fastforward.PathForce).Spec().Skip)
	assert.Equal(t, domain.SkipSafe, fieldOf(w.questions(t, fastforward.Request{}).Form(), fastforward.PathForce).Spec().Skip)
}

func TestEveryWorktreeSkipsThePickerAndRefusesAName(t *testing.T) {
	w := newWorld(t)

	assert.Equal(t, domain.SkipAll, fieldOf(w.questions(t, fastforward.Request{All: true}).Form(), domain.PathWorktree).Spec().Skip)
	assert.Equal(t, map[string][]string{fastforward.PathAll: {fastforward.CodeAllAndNames}}, fieldCodes(w.questions(t, fastforward.Request{All: true, Worktrees: []string{behind}}).Errors))
}

func TestEachEntryItCannotTakeIsAProblemOfItsOwn(t *testing.T) {
	w := newWorld(t)

	q := w.questions(t, naming(behind, " ", behind, "feat", loose, w.at("detached"), w.at(latest), latest))

	assert.Equal(t, map[string][]string{
		"worktree[1]": {domain.CodeRequired},
		"worktree[2]": {domain.CodeDistinct},
		"worktree[3]": {domain.CodeNotFound},
		"worktree[4]": {domain.CodeNotFound},
		"worktree[5]": {fastforward.CodeDetached},
		"worktree[7]": {domain.CodeDistinct},
	}, fieldCodes(q.Errors))
}

func TestNamingNoneIsNoProblemOfTheCore(t *testing.T) {
	w := newWorld(t)

	assert.Empty(t, w.questions(t, fastforward.Request{}).Errors)
}

func TestTheRequestRoundTripsThroughItsForm(t *testing.T) {
	req := fastforward.Request{Worktrees: []string{behind, dirty}, All: true, Force: true}

	answered, err := domain.Fold(fastforward.Request{}, domain.AnswersOf(fastforward.Questions{}.Form(), req))

	require.NoError(t, err)
	assert.Equal(t, req, answered)
}
