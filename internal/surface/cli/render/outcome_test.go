package render_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

func TestASuccessfulRunContractsToItsCount(t *testing.T) {
	var out bytes.Buffer

	render.Outcome(&out, render.OutcomeParams{Catalog: catalog, Command: "make", Items: []kernel.Item[json.RawMessage]{done("feat/a"), done("feat/b"), unchanged("feat/c")}, Detail: render.GenericDetail{}})

	assert.Equal(t, lines("  ✓ make: 2 done · 1 unchanged"), plain(out.String()))
}

func TestEveryItemThatIsNotDoneIsNamedWithWhy(t *testing.T) {
	var out bytes.Buffer
	failure := kernel.Internal(kernel.Problem{Code: kernel.CodeStepFailed, Params: kernel.Params{kernel.ParamStep: "worktree"}})

	render.Outcome(&out, render.OutcomeParams{Catalog: catalog,
		Command: "make",
		Items: []kernel.Item[json.RawMessage]{
			done("feat/a"),
			{Subject: "feat/b", Status: kernel.StatusFailed, Error: failure},
			{Subject: "feat/c", Status: kernel.StatusSkipped, Reason: kernel.ReasonNotReached},
		},
		Warnings:  []kernel.Warning{{Code: kernel.CodeLocksUnavailable}},
		FollowUps: []render.NextStepParams{{Command: "wtm make feat/b", Note: "retry"}},
		Detail:    render.GenericDetail{},
	})

	assert.Equal(t, lines(
		"  ✗ make: 1 done · 1 failed · 1 skipped",
		"",
		"  ! ran without operation locks",
		"",
		"  ✗ feat/b  worktree failed",
		"  ! feat/c  not reached",
		"",
		"  → wtm make feat/b   retry",
	), plain(out.String()))
}

func TestTheHeadlineCarriesTheWorstResult(t *testing.T) {
	cases := []struct {
		items []kernel.Item[json.RawMessage]
		want  string
	}{
		{nil, "  = make: nothing to do\n"},
		{[]kernel.Item[json.RawMessage]{unchanged("a")}, "  = make: 1 unchanged\n"},
		{[]kernel.Item[json.RawMessage]{done("a"), {Subject: "b", Status: kernel.StatusCancelled, Reason: kernel.ReasonInterrupted}}, "  ! make: 1 done · 1 cancelled\n\n  ! b  interrupted\n"},
	}
	for _, c := range cases {
		var out bytes.Buffer
		render.Outcome(&out, render.OutcomeParams{Catalog: catalog, Command: "make", Items: c.items, Detail: render.GenericDetail{}})
		assert.Equal(t, c.want, plain(out.String()))
	}
}

func TestTheConclusionIsThePlanAppliedWithWhatEachItemAdds(t *testing.T) {
	var out bytes.Buffer

	render.Outcome(&out, render.OutcomeParams{
		Catalog: catalog, Command: "make", Plan: json.RawMessage(`{"from":"main"}`),
		Items:  []kernel.Item[json.RawMessage]{done("feat/a"), done("feat/b")},
		Detail: planAndPath{},
	})

	assert.Equal(t, lines(
		"  ✓ make: 2 done",
		"",
		"  From    main",
		"  feat/a  \"/wt/feat/a\"",
		"  feat/b  \"/wt/feat/b\"",
		"",
		"  → wtm go feat/a",
		"  → wtm go feat/b",
	), plain(out.String()))
}

func TestAWarningAboutAnItemIsNamedWithItsSubject(t *testing.T) {
	var out bytes.Buffer
	item := kernel.ItemFor[json.RawMessage]("feat/a").Warned(kernel.Warning{Code: kernel.CodeLocksUnavailable}).Done(nil)

	render.Outcome(&out, render.OutcomeParams{Catalog: catalog, Command: "make", Items: []kernel.Item[json.RawMessage]{item}, Detail: render.GenericDetail{}})

	assert.Equal(t, lines(
		"  ✓ make: 1 done",
		"",
		"  ! feat/a  ran without operation locks",
	), plain(out.String()))
}

func TestAnItemWhoseSagaStandsButAPhaseDidNotFinishSaysWhy(t *testing.T) {
	var out bytes.Buffer
	item := kernel.Item[json.RawMessage]{Subject: "feat/a", Status: kernel.StatusCancelled, Reason: kernel.ReasonIncomplete}

	render.Outcome(&out, render.OutcomeParams{Catalog: catalog, Command: "make", Items: []kernel.Item[json.RawMessage]{item}, Detail: render.GenericDetail{}})

	assert.Contains(t, plain(out.String()), "feat/a  left incomplete")
}

func TestAnIncompleteItemSaysWhyAndIsShownByItsDetailSinceItStands(t *testing.T) {
	var out bytes.Buffer
	failure := kernel.Cancelled(kernel.Problem{Code: kernel.CodeInterrupted})
	item := kernel.Item[json.RawMessage]{Subject: "feat/a", Status: kernel.StatusCancelled, Reason: kernel.ReasonIncomplete, Error: failure, Detail: json.RawMessage(`"/wt/feat/a"`)}

	render.Outcome(&out, render.OutcomeParams{Catalog: catalog, Command: "make", Items: []kernel.Item[json.RawMessage]{item}, Detail: pathDetail{}})

	assert.Equal(t, lines(
		"  ! make: 1 cancelled",
		"",
		"  feat/a  \"/wt/feat/a\"",
		"",
		"  ! feat/a  interrupted · left incomplete",
		"",
		"  → wtm go feat/a",
	), plain(out.String()))
}
