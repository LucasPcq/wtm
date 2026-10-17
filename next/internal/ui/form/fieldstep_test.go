package form

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// asking opens the wizard of the fake command on the step at, every step before it given.
func asking(t *testing.T, a *app, given request) (*driver, *Session[request, questions, outcome]) {
	t.Helper()
	s := fakeCommand().Session(t.Context(), SessionParams[request, questions, outcome]{App: a, Request: given})
	return start(New(t.Context(), s.Params())), s
}

func TestATextListBadgesItsEntriesAndRefusesOneAsItIsAdded(t *testing.T) {
	d, s := asking(t, &app{}, nil)

	d.keys("feature", "tab", "taken", "tab")

	screen := d.screen()
	assert.Contains(t, screen, "feature  existing")
	assert.Contains(t, screen, "✗ taken is taken")
	d.keys("backspace", "backspace", "backspace", "backspace", "backspace", "enter")
	assert.Equal(t, []string{"feature"}, s.Request()["branches"])
	assert.Contains(t, d.screen(), "✓ Branches: feature")
}

func TestASelectPinsItsOptionAndRefreshes(t *testing.T) {
	d, _ := asking(t, &app{}, request{"branches": {"feat/a"}})

	screen := d.screen()
	assert.Contains(t, screen, "Step 2/8 • Source")
	assert.Contains(t, screen, "▌› dev")
	assert.Contains(t, screen, "r refresh")
}

func TestASelectStartsOnItsRememberedAnswerItsToggleTicked(t *testing.T) {
	d, s := asking(t, &app{remembered: "slow"}, request{"branches": {"feat/a"}, "from": {"main"}})

	assert.Contains(t, d.screen(), "▌› slow")
	assert.Contains(t, d.screen(), "[x] Always use this answer in this repo")
	d.keys("enter")

	assert.Equal(t, request{"branches": {"feat/a"}, "from": {"main"}, "mode": {"slow"}, domain.PathRemember: {"mode"}}, s.Request())
	assert.Contains(t, d.screen(), "✓ Mode: slowly")
}

func TestARememberableConfirmationSetsItsAnswersApartAboveTheToggle(t *testing.T) {
	d, s := asking(t, &app{}, request{"branches": {"feat/a"}, "from": {"main"}, "mode": {"fast"}})

	assert.Contains(t, d.screen(), "   Yes\n  ────")
	assert.Contains(t, d.screen(), "▌› No\n\n   [ ] Always use this answer in this repo")
	d.keys("up", "tab", "enter")

	assert.Equal(t, []string{"true"}, s.Request()["ff"])
	assert.Equal(t, []string{"ff"}, s.Request()[domain.PathRemember])
	assert.Contains(t, d.screen(), "✓ Update: yes")
}

func TestAConfirmationListsItsOptionsAsTheyReadNoFirst(t *testing.T) {
	d, s := asking(t, &app{}, request{"branches": {"feat/a"}, "from": {"main"}, "mode": {"fast"}, "ff": {"false"}})

	assert.Contains(t, d.screen(), "▌› Move\n   Copy\n")
	assert.NotContains(t, d.screen(), "Always use")
	d.keys("down", "enter")

	assert.Equal(t, []string{"true"}, s.Request()["keep"])
	assert.Contains(t, d.screen(), "✓ Keep: copy")
}

func TestAMultiSelectRefusesNoneWithItsFieldsProblem(t *testing.T) {
	d, s := asking(t, &app{}, request{"branches": {"feat/a"}, "from": {"main"}, "mode": {"fast"}, "ff": {"false"}, "keep": {"false"}})

	d.keys("enter")
	assert.Contains(t, d.screen(), "✗ Pick one")
	d.keys("space", "enter")

	assert.Equal(t, []string{"a.txt"}, s.Request()["files"])
}

func TestGoingBackForgetsTheAnswerOfEachStepLeftAndKeepsTheOneReturnedTo(t *testing.T) {
	d, s := asking(t, &app{}, request{"branches": {"feat/a"}})
	d.keys("down", "enter", "enter")
	require.Equal(t, []string{"fast"}, s.Request()["mode"])

	d.keys("esc", "esc")

	assert.Equal(t, atFrom, d.w.Current())
	assert.Equal(t, request{"branches": {"feat/a"}, "from": {"main"}}, s.Request())
}
