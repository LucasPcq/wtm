package form

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
)

func TestAFlagSettlesItsFieldWithTheFlagThatGaveIt(t *testing.T) {
	d := newWorld(t, &repo{}, worldParams{Request: `{"branches":["feat/a"]}`}).open(t)

	assert.Equal(t, []string{"Branches: feat/a · --branches"}, d.trail())
	assert.Contains(t, d.view(), "Source")
}

func TestARememberedAnswerIsSettled(t *testing.T) {
	d := newWorld(t, &repo{remembered: verbatim}, worldParams{Request: `{"branches":["feat/a"],"from":"main"}`}).open(t)

	assert.Contains(t, d.trail(), "Isolation: verbatim"+domain.RecapRememberedSuffix)
	assert.Contains(t, d.view(), "Push")
}

func TestAskAsksARememberedAnswerAgainTickedOnIt(t *testing.T) {
	d := newWorld(t, &repo{remembered: verbatim}, worldParams{Request: `{"branches":["feat/a"],"from":"main"}`, Options: dispatch.Options{IgnoreRemembered: true}}).open(t)

	assert.Contains(t, d.view(), domain.DashboardGlyphCheckOn+" "+domain.RememberToggleLabel)
	d.key("enter")
	assert.Contains(t, d.trail(), "Isolation: verbatim")
}

func TestTickingAlwaysUseNamesTheFieldToRemember(t *testing.T) {
	w := newWorld(t, &repo{}, worldParams{Request: `{"branches":["feat/a"],"from":"main"}`})
	d := w.open(t)

	d.key("tab", "enter")
	for range 4 {
		d.key("enter")
	}

	assert.Equal(t, []string{"isolation"}, requestOf(t, w.session.request).Remember)
}

func TestAFieldADefaultOffersIsAskedAndKeptWithItsOrigin(t *testing.T) {
	d := newWorld(t, &repo{}, worldParams{Request: `{"branches":["feat/a"]}`}).open(t)

	d.key("enter")

	assert.Contains(t, d.trail(), "Source: main · config")
}

func TestAFieldItsConditionRulesOutSaysWhy(t *testing.T) {
	d := newWorld(t, &repo{}, worldParams{Request: `{"branches":["feat/a"],"from":"main","isolation":"isolated"}`}).open(t)

	d.key("enter")

	assert.Contains(t, d.trail(), "Message — test.no_push")
	assert.Contains(t, d.view(), "Hooks")
}

func TestAnswerYesAndTheFieldThatDependsOnItIsAsked(t *testing.T) {
	d := newWorld(t, &repo{}, worldParams{Request: `{"branches":["feat/a"],"from":"main","isolation":"isolated"}`}).open(t)

	d.key("up", "enter")

	assert.Contains(t, d.trail(), "Push: yes")
	assert.Contains(t, d.view(), "Message")
}

func TestAnInvalidValueAFlagGaveIsAskedAgain(t *testing.T) {
	d := newWorld(t, &repo{}, worldParams{Request: `{"branches":["Not A Branch"]}`}).open(t)

	assert.Empty(t, d.trail())
	assert.Contains(t, d.view(), "Not A Branch")
}

func TestTheSecondPassAfterStaleFactsSettlesWhatWasAnswered(t *testing.T) {
	w := newWorld(t, &repo{}, worldParams{})
	w.session.flagged = map[string]bool{}
	w.session.answered = map[string]bool{"branches": true, "from": true, "isolation": true, "push": true, "hooks": true, "order": true, "env": true}
	w.session.given = []byte(`{"branches":["feat/a"],"from":"main","isolation":"isolated","hooks":[],"order":["web","api"],"env":{".env":"copy"}}`)
	w.session.stale = true

	d := w.open(t)

	require.Contains(t, d.trail(), "Branches: feat/a")
	assert.Contains(t, d.view(), recapTitle)
}
