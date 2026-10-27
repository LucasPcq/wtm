package reparent_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/reparent"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestADetachedWorktreeInTheRepositoryRefusesNoMove(t *testing.T) {
	w := newWorld(t)
	req := moving("main", featC)

	plan := reparent.Decide(w.facts(t, req), req)

	assert.Equal(t, []reparent.Move{{Branch: featC, Path: w.at(featC), From: featB, To: "main"}}, plan.Moves)
}

func TestACycleElsewhereRefusesNothingAndAMemberCanMoveOutOfIt(t *testing.T) {
	w := newWorld(t)
	w.recordParent(t, featA, featB)

	elsewhere := w.questions(t, moving(topic, featX))
	out := w.questions(t, moving("main", featA))
	into := w.questions(t, moving(featB, featA))

	assert.Empty(t, elsewhere.Errors)
	assert.Empty(t, out.Errors)
	assert.Equal(t, map[string][]string{reparent.PathTo: {reparent.CodeCycle}}, fieldCodes(into.Errors))
}

func TestAMoveCarriesTheChildrenThatFollowIt(t *testing.T) {
	w := newWorld(t)
	req := moving("main", featA)

	plan := reparent.Decide(w.facts(t, req), req)

	assert.Equal(t, []string{featB}, plan.Moves[0].Children)
}

func TestTheRecapSaysWhereEachWorktreeComesFrom(t *testing.T) {
	w := newWorld(t)

	plan, _, err := w.usecase.Plan(t.Context(), moving("main", featB, featX))

	assert.NoError(t, err)
	assert.Equal(t, []domain.RecapValue{
		{Value: featB, Params: domain.Params{"from": featA, "to": "main"}, Text: "feat/b (feat/a → main)"},
		{Value: featX, Note: reparent.NoteUnchanged, Params: domain.Params{"from": "main", "to": "main"}, Text: "feat/x (already on main)"},
	}, plan.Recap[0].Values)
	assert.Equal(t, reparent.PathTo, plan.Recap[1].Path)
}

func TestThePlanHoldsTheNewParentAndWhatItSitsUnderShared(t *testing.T) {
	w := newWorld(t)
	req := moving(featB, featC, featX)

	plan := reparent.Decide(w.facts(t, req), req)

	assert.ElementsMatch(t, []domain.Lock{
		{Scope: domain.ScopeRepo, Mode: domain.LockShared},
		{Scope: domain.ScopeWorktree, Name: featC, Mode: domain.LockExclusive},
		{Scope: domain.ScopeWorktree, Name: featX, Mode: domain.LockExclusive},
		{Scope: domain.ScopeWorktree, Name: featB, Mode: domain.LockShared},
		{Scope: domain.ScopeWorktree, Name: featA, Mode: domain.LockShared},
		{Scope: domain.ScopeWorktree, Name: "main", Mode: domain.LockShared},
	}, plan.Locks())
}
