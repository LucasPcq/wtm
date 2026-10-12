package form

import (
	"encoding/json"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/styles"
)

func TestARecapWithoutAPlanSaysWhyAsAnErrorAndOffersOnlyToCancel(t *testing.T) {
	w := newWorld(t, &repo{}, worldParams{})
	w.session.failure = kernel.Invalid([]kernel.FieldError{{Path: "branches[1]", Code: kernel.CodeDistinct, Params: kernel.Params{kernel.ParamValue: "feat/a"}}})

	view := w.recapView(t)

	assert.Contains(t, view, failedTitle)
	assert.Contains(t, view, domain.WizardCancelLabel)
	assert.NotContains(t, view, applyLabel)
	assert.Equal(t, "✗ the request is invalid\n    Branches[1]: feat/a is given twice", ansi.Strip(w.session.describe(w.session.failure)))
}

func TestTheRecapIsThePlanAsTheCommandDrawsItThenItsWarningsApart(t *testing.T) {
	w := newWorld(t, &repo{}, worldParams{})
	w.session.preview = previewOf(`{"branches":["feat/a"]}`, kernel.Warning{Code: kernel.CodeLocksUnavailable})

	view := w.recapView(t)

	assert.Contains(t, view, recapTitle)
	assert.Contains(t, view, applyLabel)
	assert.Contains(t, view, domain.WizardCancelLabel)
	assert.Equal(t, "Plan  {\"branches\":[\"feat/a\"]}\n\n! ran without operation locks", ansi.Strip(w.session.recapText(w.session.params.Plan(w.session.preview.Plan.Detail))))
}

func TestAPlanThatDoesNothingOffersOnlyToCancel(t *testing.T) {
	w := newWorld(t, &repo{}, worldParams{})
	w.session.params.Plan = func(json.RawMessage) []styles.PlanRow { return nil }
	w.session.preview = previewOf(`{}`)

	view := w.recapView(t)

	assert.Contains(t, view, "= nothing to do")
	assert.NotContains(t, view, applyLabel)
	assert.Contains(t, view, domain.WizardCancelLabel)
}

func TestARecapOnFactsThatChangedSaysSoAboveTheNewPlan(t *testing.T) {
	w := newWorld(t, &repo{}, worldParams{})
	w.session.stale = true
	w.session.preview = previewOf(`{"branches":["feat/a"]}`)

	text := ansi.Strip(w.session.recapText(w.session.params.Plan(w.session.preview.Plan.Detail)))

	assert.Equal(t, "! the facts changed since the preview\n\nPlan  {\"branches\":[\"feat/a\"]}", text)
}

func TestChoicesThatFailEndTheRunWithTheirError(t *testing.T) {
	w := newWorld(t, &repo{choicesErr: errListing}, worldParams{})

	_, err := w.script(t, func(d *driver) {
		d.typed("feat/a")
		d.key("enter")
	})

	assert.ErrorIs(t, err, errListing)
}

func TestTheRecapIsDrawnAgainAtTheWidthOfTheTerminal(t *testing.T) {
	w := newWorld(t, &repo{}, worldParams{})
	w.session.params.Plan = func(json.RawMessage) []styles.PlanRow {
		return []styles.PlanRow{{Label: "Branches", Values: []string{"feat/authentication", "feat/billing", "fix/session"}}}
	}
	d := w.open(t)
	fillAll(d)

	d.send(tea.WindowSizeMsg{Width: 40, Height: 60})

	assert.Contains(t, ansi.Strip(d.view()), "┃            feat/billing")
}
