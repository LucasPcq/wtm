package render_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

func TestASuccessfulRunContractsToItsCount(t *testing.T) {
	var out bytes.Buffer

	render.Outcome(&out, render.OutcomeParams{Command: "make", Items: []kernel.Item[json.RawMessage]{done("feat/a"), done("feat/b"), unchanged("feat/c")}, Detail: render.GenericDetail{}})

	assert.Equal(t, lines("  ✓ make: 2 done · 1 unchanged"), plain(out.String()))
}

func TestEveryItemThatIsNotDoneIsNamedWithWhy(t *testing.T) {
	var out bytes.Buffer
	failure := kernel.Internal(kernel.Problem{Code: kernel.CodeStepFailed, Params: kernel.Params{kernel.ParamStep: "worktree"}})

	render.Outcome(&out, render.OutcomeParams{
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
		"  ✗ feat/b  worktree failed",
		"  ! feat/c  not reached",
		"  ! ran without operation locks",
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
		{[]kernel.Item[json.RawMessage]{done("a"), {Subject: "b", Status: kernel.StatusCancelled, Reason: kernel.ReasonInterrupted}}, "  ! make: 1 done · 1 cancelled\n  ! b  interrupted\n"},
	}
	for _, c := range cases {
		var out bytes.Buffer
		render.Outcome(&out, render.OutcomeParams{Command: "make", Items: c.items, Detail: render.GenericDetail{}})
		assert.Equal(t, c.want, plain(out.String()))
	}
}

func TestADoneItemIsShownByTheCommandsOwnDetail(t *testing.T) {
	var out bytes.Buffer

	render.Outcome(&out, render.OutcomeParams{Command: "make", Items: []kernel.Item[json.RawMessage]{done("feat/a"), unchanged("feat/b")}, Detail: pathDetail{}})

	assert.Equal(t, lines("  ✓ make: 1 done · 1 unchanged", "    feat/a  \"/wt/feat/a\""), plain(out.String()))
}

func TestTheGenericPlanIsItsFieldsInTheirOwnOrder(t *testing.T) {
	var out bytes.Buffer

	render.GenericDetail{}.Plan(&out, json.RawMessage(`{"isolation":"isolated","branches":["feat/a","feat/b"],"count":2,"nested":{"a":1}}`))

	assert.Equal(t, lines(
		"  isolation  isolated",
		"  branches   feat/a, feat/b",
		"  count      2",
		`  nested     {"a":1}`,
	), plain(out.String()))
}

func TestAPlanThatIsNotAnObjectIsOneLine(t *testing.T) {
	var out bytes.Buffer

	render.GenericDetail{}.Plan(&out, json.RawMessage(`"feat/a"`))

	assert.Equal(t, "    feat/a\n", plain(out.String()))
}

type pathDetail struct{}

func (pathDetail) Plan(io.Writer, json.RawMessage) {}

func (pathDetail) Item(w io.Writer, item kernel.Item[json.RawMessage]) {
	fmt.Fprintf(w, "    %s  %s\n", item.Subject, item.Detail)
}
