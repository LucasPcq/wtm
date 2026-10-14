package form

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestAnEntryStateRefusesStaysInTheListWithWhy(t *testing.T) {
	d := newWorld(t, &repo{}, worldParams{}).open(t)

	d.typed("Not A Branch")
	d.key("enter")

	assert.Contains(t, d.view(), "does not match")
	assert.Empty(t, d.trail())
}

func TestAnEntryIsBadgedAsItsChoiceIs(t *testing.T) {
	d := newWorld(t, &repo{existing: []string{"feat/old"}}, worldParams{}).open(t)

	d.typed("feat/old")
	d.key("tab")

	assert.Contains(t, d.view(), string(existsBadge))
}

func TestAnOptionARuleRefusesArrivesDisabledWithWhy(t *testing.T) {
	d := newWorld(t, &repo{existing: []string{"feat/a"}}, worldParams{Request: `{"branches":["feat/a"]}`}).open(t)

	assert.Contains(t, d.view(), "feat/a cannot be its own parent")
	d.key("down", "enter")
	assert.Contains(t, d.trail(), "Source: main · config", "the refused option cannot be picked")
}

func TestEachKeyOfADecisionsFieldCyclesAmongItsChoices(t *testing.T) {
	w := newWorld(t, &repo{}, worldParams{Request: `{"branches":["feat/a"],"from":"main","isolation":"isolated","hooks":["install"],"order":["api"]}`, Options: dispatchDryRun()})
	d := w.open(t)
	d.key("enter")

	assert.Contains(t, d.view(), "‹ copy ›")
	d.key("right", "enter")
	assert.Equal(t, map[string]string{envFile: linked}, requestOf(t, w.session.requestFrom(d.model.Steps())).Env)
}

func TestThePinnedChoiceComesFirst(t *testing.T) {
	found := options(kernel.Choices{Options: []kernel.Option{{Value: "feat/old"}, {Value: base}}, Pinned: base}, nil)

	assert.Equal(t, []kernel.Option{{Value: base}, {Value: "feat/old"}}, found)
}

func TestAFieldWithoutChoicesOffersWhatItsEnumAccepts(t *testing.T) {
	assert.Equal(t, []kernel.Option{{Value: isolated, Label: isolated}}, options(kernel.Choices{}, []string{isolated}))
}
