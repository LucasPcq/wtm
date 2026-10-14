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
