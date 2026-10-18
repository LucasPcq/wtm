package form_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
)

// asking opens the wizard of the fake command on the step at, every step before it given.
func asking(t *testing.T, a *app, given request) (*formtest.Driver, *form.Session[request, questions, outcome]) {
	t.Helper()
	s := fakeCommand().Session(t.Context(), form.SessionParams[request, questions, outcome]{App: a, Request: given})
	return start(form.New(t.Context(), s.Params())), s
}

func TestATextListBadgesItsEntriesAndRefusesOneAsItIsAdded(t *testing.T) {
	d, s := asking(t, &app{}, nil)

	d.Keys("feature", "tab", "taken", "tab")

	screen := d.Screen()
	assert.Contains(t, screen, "feature  existing")
	assert.Contains(t, screen, "✗ taken is taken")
	d.Keys("backspace", "backspace", "backspace", "backspace", "backspace", "enter")
	assert.Equal(t, []string{"feature"}, s.Request()["branches"])
	assert.Contains(t, d.Screen(), "✓ Branches: feature")
}

func TestATextListBadgesAnEntryItDoesNotOfferAsNew(t *testing.T) {
	d, _ := asking(t, &app{}, nil)

	d.Keys("feat/a", "tab")

	assert.Contains(t, d.Screen(), "feat/a  new")
}

func TestASelectPinsItsOptionAndRefreshes(t *testing.T) {
	d, _ := asking(t, &app{}, request{"branches": {"feat/a"}})

	screen := d.Screen()
	assert.Contains(t, screen, "Step 2/8 • Source")
	assert.Contains(t, screen, "▌› dev")
	assert.Contains(t, screen, "r refresh")
}

func TestASelectStartsOnItsRememberedAnswerItsToggleTicked(t *testing.T) {
	d, s := asking(t, &app{remembered: "slow"}, request{"branches": {"feat/a"}, "from": {"main"}})

	assert.Contains(t, d.Screen(), "▌› slow")
	assert.Contains(t, d.Screen(), "[x] Always use this answer in this repo")
	d.Keys("enter")

	assert.Equal(t, request{"branches": {"feat/a"}, "from": {"main"}, "mode": {"slow"}, domain.PathRemember: {"mode"}}, s.Request())
	assert.Contains(t, d.Screen(), "✓ Mode: slowly")
}

func TestARememberableConfirmationSetsItsAnswersApartAboveTheToggle(t *testing.T) {
	d, s := asking(t, &app{}, request{"branches": {"feat/a"}, "from": {"main"}, "mode": {"fast"}})

	assert.Contains(t, d.Screen(), "   Yes\n  ────")
	assert.Contains(t, d.Screen(), "▌› No\n\n   [ ] Always use this answer in this repo")
	d.Keys("up", "tab", "enter")

	assert.Equal(t, []string{"true"}, s.Request()["ff"])
	assert.Equal(t, []string{"ff"}, s.Request()[domain.PathRemember])
	assert.Contains(t, d.Screen(), "✓ Update: yes")
}

func TestAConfirmationListsItsOptionsAsTheyReadNoFirst(t *testing.T) {
	d, s := asking(t, &app{}, request{"branches": {"feat/a"}, "from": {"main"}, "mode": {"fast"}, "ff": {"false"}})

	assert.Contains(t, d.Screen(), "▌› Move\n   Copy\n")
	assert.NotContains(t, d.Screen(), "Always use")
	d.Keys("down", "enter")

	assert.Equal(t, []string{"true"}, s.Request()["keep"])
	assert.Contains(t, d.Screen(), "✓ Keep: copy")
}

func TestAMultiSelectRefusesNoneWithItsFieldsProblem(t *testing.T) {
	d, s := asking(t, &app{}, request{"branches": {"feat/a"}, "from": {"main"}, "mode": {"fast"}, "ff": {"false"}, "keep": {"false"}})

	d.Keys("enter")
	assert.Contains(t, d.Screen(), "✗ Pick one")
	d.Keys("space", "enter")

	assert.Equal(t, []string{"a.txt"}, s.Request()["files"])
}

func TestGoingBackForgetsTheAnswerOfEachStepLeftAndKeepsTheOneReturnedTo(t *testing.T) {
	d, s := asking(t, &app{}, request{"branches": {"feat/a"}})
	d.Keys("down", "enter", "enter")
	require.Equal(t, []string{"fast"}, s.Request()["mode"])

	d.Keys("esc", "esc")

	assert.Equal(t, atFrom, d.Wizard.Current())
	assert.Equal(t, request{"branches": {"feat/a"}, "from": {"main"}}, s.Request())
}
