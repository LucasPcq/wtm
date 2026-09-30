package components

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// IsolationStep is the isolation question for the wizards not yet on flow/,
// worded and ordered like internal/flow/create's so the three create-like
// commands put the same question the same way.
func IsolationStep(first domain.Isolation) Step {
	choices := rules.IsolationChoices(first)
	items := make([]SelectItem, 0, len(choices))
	for _, choice := range choices {
		items = append(items, SelectItem{Label: rules.IsolationOptionLabel(choice), Value: string(choice)})
	}
	return Step{
		Name: domain.IsolationStepName,
		Model: NewSelectList(NewSelectListParams{
			Title:       domain.IsolationStepName,
			Description: domain.IsolationStepDescription,
			Items:       items,
		}),
		Summary: func(model any) string {
			list, ok := model.(SelectListModel)
			if !ok {
				return ""
			}
			return rules.IsolationSummary(domain.Isolation(list.Value()))
		},
	}
}

// IsolationAnswer reads the step's answer from a finished wizard, falling back
// to what resolved it when the step was not in it.
func IsolationAnswer(steps []Step, fallback domain.Isolation) domain.Isolation {
	for _, step := range steps {
		if step.Name != domain.IsolationStepName {
			continue
		}
		list, ok := step.Model.(SelectListModel)
		if ok && list.Value() != "" {
			return domain.Isolation(list.Value())
		}
	}
	return rules.EffectiveIsolation(fallback)
}
