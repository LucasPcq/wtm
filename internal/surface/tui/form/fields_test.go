package form

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

func TestEveryFieldTypeIsReadBackAsTheValueOfItsField(t *testing.T) {
	cases := map[string]struct {
		model any
		want  kernel.Value
	}{
		"text":        {components.NewTextInput(components.NewTextInputParams{Default: "fix"}), kernel.Text("fix")},
		"bool":        {components.NewConfirm(components.NewConfirmParams{}), kernel.Bool(false)},
		"select":      {components.NewSelectList(components.NewSelectListParams{Items: []components.SelectItem{{Value: base}}}), kernel.Text(base)},
		"multiselect": {components.NewMultiSelect(components.NewMultiSelectParams{Items: []components.MultiSelectItem{{Value: "install", Selected: true}}}), kernel.List{"install"}},
		"reorder":     {components.NewReorderList(components.NewReorderListParams{Items: []components.ReorderItem{{Value: "api"}, {Value: "web"}}}), kernel.List{"api", "web"}},
		"textlist":    {components.NewTextList(components.NewTextListParams{Entries: []string{"feat/a"}}), kernel.List{"feat/a"}},
		"decisions":   {components.NewDecisionList(components.NewDecisionListParams{Rows: []components.DecisionRow{{Key: envFile, Options: []string{copied, linked}}}}), kernel.Decisions{envFile: copied}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, valueOf(c.model))
		})
	}
}

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
