package form

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestARecapWithoutAPlanSaysWhyAndOffersOnlyToCancel(t *testing.T) {
	w := newWorld(t, &repo{}, worldParams{})
	w.session.failure = kernel.Invalid([]kernel.FieldError{{Path: "branches[1]", Code: kernel.CodeDistinct, Params: kernel.Params{kernel.ParamValue: "feat/a"}}})

	recap := w.session.recapModel()

	assert.Contains(t, recap.View(), domain.WizardCancelLabel)
	assert.NotContains(t, recap.View(), applyLabel)
	assert.Equal(t, "the request is invalid\nBranches[1]: feat/a is given twice", w.session.describe(w.session.failure))
}

func TestTheRecapIsThePlanAsTheCommandDrawsItThenItsWarnings(t *testing.T) {
	w := newWorld(t, &repo{}, worldParams{})
	w.session.preview = &dispatch.CommandReply{Result: dispatch.Result[json.RawMessage, json.RawMessage]{
		Plan: kernel.Plan[json.RawMessage]{Detail: json.RawMessage(`{"branches":["feat/a"]}`), Warnings: []kernel.Warning{{Code: kernel.CodeLocksUnavailable}}},
	}}

	view := w.session.recapModel().View()

	assert.Contains(t, view, applyLabel)
	assert.Contains(t, view, domain.WizardCancelLabel)
	assert.Equal(t, "Plan  {\"branches\":[\"feat/a\"]}\n"+domain.GlyphAttention+" ran without operation locks", ansi.Strip(w.session.recapText()))
}

func TestChoicesThatFailEndTheRunWithTheirError(t *testing.T) {
	w := newWorld(t, &repo{choicesErr: errListing}, worldParams{})

	_, err := w.script(t, func(d *driver) {
		d.typed("feat/a")
		d.key("enter")
	})

	assert.ErrorIs(t, err, errListing)
}
