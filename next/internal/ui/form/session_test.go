package form

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	atBranches = iota
	atFrom
	atMode
	atFF
	atKeep
	atFiles
	atName
)

func TestAStepGivenByAFlagSaysWhichAndOneLeftOpenIsAsked(t *testing.T) {
	s := observed(t, &app{}, request{"from": {"main"}})

	assert.Equal(t, Standing{Value: "main", Flag: "--from"}, s.Standing(atFrom))
	assert.Equal(t, Standing{Value: "move"}, s.Standing(atKeep))
	assert.Equal(t, Standing{Value: "feat/a", Flag: "<branches>"}, observed(t, &app{}, request{"branches": {"feat/a"}}).Standing(atBranches))
}

func TestAStepARefusedEntryNamesIsErred(t *testing.T) {
	s := observed(t, &app{}, request{"branches": {"feat/a", "taken"}})

	assert.True(t, s.Standing(atBranches).Erred)
	assert.False(t, s.Standing(atFrom).Erred)
}

func TestTheRequestIsTheOneGivenWithEachAnswerInStepOrder(t *testing.T) {
	s := observed(t, &app{}, request{"from": {"main"}, domain.PathRemember: {"ff"}})

	s.remembering(atMode, "mode", true, domain.Answered{Path: "mode", Values: []string{"slow"}})
	s.Answer(atFrom, domain.Answered{Path: "from", Values: []string{"dev"}})
	s.remembering(atFF, "ff", false, domain.Answered{Path: "ff", Values: []string{"true"}})

	assert.Equal(t, request{"from": {"dev"}, "mode": {"slow"}, "ff": {"true"}, domain.PathRemember: {"mode"}}, s.Request())
}

func TestForgettingAStepReturnsItsFieldsToWhatWasGiven(t *testing.T) {
	s := observed(t, &app{}, request{"from": {"main"}, domain.PathRemember: {"ff"}})
	s.Answer(atFrom, domain.Answered{Path: "from", Values: []string{"dev"}})
	s.remembering(atFF, "ff", false, domain.Answered{Path: "ff", Values: []string{"true"}})

	s.Forget(atFF)
	s.Forget(atFrom)

	assert.Equal(t, request{"from": {"main"}, domain.PathRemember: {"ff"}}, s.Request())
}

func TestAPathIsLabelledByItsFirstFieldKeepingAnEntrysIndex(t *testing.T) {
	s := observed(t, &app{}, nil)

	assert.Equal(t, "Source", s.Label("from"))
	assert.Equal(t, "Branches[1]", s.Label("branches[1]"))
	assert.Equal(t, "remember[0]", s.Label("remember[0]"))
}

func TestTheRecapAppliesThePlanItPreviewedHeldToItsFingerprint(t *testing.T) {
	a := &app{}
	s := fakeCommand().Session(t.Context(), SessionParams[*app, request]{App: a, Request: request{"branches": {"feat/a"}, "from": {"main"}, "mode": {"fast"}, "ff": {"false"}, "keep": {"false"}, "files": {"a.txt"}, "name": {"x"}}, Options: domain.RunOptions{CorrelationID: "corr-1"}})
	d := start(New(t.Context(), s.Params()))
	require.Contains(t, d.screen(), "plan of feat/a")

	d.keys("enter")
	require.NoError(t, s.apply(t.Context()))

	require.Len(t, a.runs, 2)
	assert.Equal(t, domain.RunOptions{DryRun: true, Surface: "tui", CorrelationID: "corr-1"}, a.runs[0])
	assert.Equal(t, domain.RunOptions{ExpectFingerprint: "print", Surface: "tui", CorrelationID: "corr-1"}, a.runs[1])
	assert.Equal(t, "print", s.outcome.PlanFingerprint())
}
