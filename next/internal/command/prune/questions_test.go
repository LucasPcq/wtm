package prune_test

import (
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/prune"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheWorktreesOfferedAreThoseDoneWithAndTheSafeOnesArePicked(t *testing.T) {
	w := newWorld(t, seed{})

	questions := w.questions(t, prune.Request{})

	field, ok := fieldOf(questions.Form(), domain.PathWorktree).(domain.MultiSelectField)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{merged, closed, gone, goneWork, dirty}, namesOf(questions.Candidates))
	assert.ElementsMatch(t, []string{merged, closed, gone}, field.Picked)
	assert.Len(t, field.Options, 5)
	assert.Equal(t, domain.SkipSafe, questions.Force.Skip, "an unsafe one not picked is kept, nothing to force")
}

// B3: legacy's wizard listed an unsafe worktree, let it be checked, then dropped it silently after the confirmation.
func TestPickingAnUnsafeWorktreeAsksWhetherToForceIt(t *testing.T) {
	w := newWorld(t, seed{})

	questions := w.questions(t, pruning(merged, dirty))

	assert.Empty(t, questions.Force.Skip)
	assert.Equal(t, []string{dirty}, namesOf(questions.Unsafe))
	assert.Equal(t, []worktree.Blocker{worktree.BlockerDirty}, questions.Unsafe[0].Blockers)
}

func TestReparentIsAskedOnlyWhenAChildIsLeftBehind(t *testing.T) {
	w := newWorld(t, seed{})

	alone := w.questions(t, pruning(closed))
	parenting := w.questions(t, pruning(merged))

	assert.Equal(t, domain.SkipNoOrphan, alone.Reparent.Skip)
	assert.Empty(t, parenting.Reparent.Skip)
	assert.Equal(t, child, parenting.Orphans[0].Branch)
}

func TestNoWorktreeDoneWithIsNotAsked(t *testing.T) {
	w := newWorld(t, seed{})

	questions := w.questions(t, prune.Request{Closed: true})
	none := w.questions(t, prune.Request{Gone: true, NoFetch: true})

	assert.Empty(t, questions.Worktrees.Skip)
	assert.Equal(t, domain.SkipNoCandidate, none.Worktrees.Skip)
}

func TestTheReasonsAndTheFetchAreSetByTheirFlagsOnly(t *testing.T) {
	w := newWorld(t, seed{})

	given := w.questions(t, prune.Request{Gone: true})

	assert.Equal(t, domain.SkipFlagOnly, given.Merged.Skip)
	assert.Equal(t, domain.SkipFlagOnly, given.NoFetch.Skip)
	assert.Empty(t, given.Gone.Skip, "a flag given never makes its line disappear")
	assert.Equal(t, domain.OriginRequest, given.Gone.Origin)
	assert.Equal(t, []prune.Reason{prune.ReasonGone}, prune.Request{Gone: true}.Reasons())
	assert.Equal(t, prune.Reasons(), prune.Request{}.Reasons())
}

func TestTheAnswersOfTheFormGiveBackTheRequest(t *testing.T) {
	w := newWorld(t, seed{})
	req := prune.Request{Worktrees: []string{merged, dirty}, ReparentChildren: true, Force: true, Merged: true, Closed: true, Gone: true, NoFetch: true}

	folded, err := domain.Fold(prune.Request{}, w.questions(t, req).Form().Answers())

	require.NoError(t, err)
	assert.Equal(t, req, folded)
	for _, path := range []string{domain.PathWorktree, prune.PathReparentChildren, prune.PathForce, prune.PathMerged, prune.PathClosed, prune.PathGone, prune.PathNoFetch} {
		again, err := prune.Request{}.Answer(path, req.Values(path))
		require.NoError(t, err)
		assert.Equal(t, req.Values(path), again.Values(path), path)
	}
}

func TestAPathNotOfTheRequestIsRefused(t *testing.T) {
	_, err := prune.Request{}.Answer("keep_data", []string{"true"})

	assert.Equal(t, map[string][]string{"keep_data": {domain.CodeUnknown}}, fieldCodes(failureOf(t, err).Fields))
}

// Narrowed to the worktrees named, the checks left every other candidate without its blockers: back in the wizard's picker, feat/dirty read as safe.
func TestEveryCandidateKeepsItsBlockersWhenSomeAreNamed(t *testing.T) {
	w := newWorld(t, seed{})

	questions := w.questions(t, pruning(merged))

	at := slices.IndexFunc(questions.Candidates, func(c prune.Candidate) bool { return c.Worktree == dirty })
	require.GreaterOrEqual(t, at, 0)
	assert.Equal(t, []worktree.Blocker{worktree.BlockerDirty}, questions.Candidates[at].Blockers)
}
