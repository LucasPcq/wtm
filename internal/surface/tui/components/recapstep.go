package components

import "github.com/LucasPcq/wtm/internal/domain"

// RecapContent is the body of a recap step: the recap text (the selections plus
// any ⚠ warnings) and the command-specific action option(s). RecapStep appends
// the constant "No, cancel" row itself, so Actions holds only the action rows.
type RecapContent struct {
	Description string
	Actions     []SelectItem
}

// RecapStepParams configures the final recap step built by RecapStep.
type RecapStepParams struct {
	// Name labels the step in the breadcrumb and completed-step summaries.
	Name string
	// Build derives the recap from the already-completed prior steps each time the
	// step is entered. RecapStep is always last and unconditional, so Build runs on
	// every entry.
	Build func(prev []Step) RecapContent
}

// RecapStep builds the final, unconditional recap step: a SelectList rendered
// with a distinct "Review & confirm" header (Recap: true) whose description
// recaps the selections and any ⚠ warnings, offering the command's action
// option(s) followed by a constant domain.WizardCancelLabel row — the single
// explicit cancellation point. The command layer reads the chosen value and maps
// domain.WizardCancelValue to domain.ErrUserAborted.
func RecapStep(p RecapStepParams) Step {
	build := func(prev []Step) any {
		content := p.Build(prev)
		items := make([]SelectItem, 0, len(content.Actions)+2)
		items = append(items, content.Actions...)
		items = append(items,
			SelectItem{Separator: true},
			SelectItem{Label: domain.WizardCancelLabel, Value: domain.WizardCancelValue},
		)
		return NewSelectList(NewSelectListParams{
			Description: content.Description,
			Items:       items,
		})
	}

	return Step{
		Name:    p.Name,
		Model:   NewSelectList(NewSelectListParams{}),
		Build:   build,
		Recap:   true,
		Summary: SelectSummary,
	}
}
