package components

// ChoiceStepParams configures a conditional select decision built by ChoiceStep.
type ChoiceStepParams struct {
	// Name labels the step in the breadcrumb and completed-step summaries.
	Name string
	// Decide runs each time the wizard enters this step. It inspects the
	// already-completed prior steps and returns whether the decision is relevant,
	// the reason to show when it is not (rendered as "⊘ <Name> — <reason>" in the
	// summaries), and the SelectList to show when it is. Every option advances the
	// wizard; there is no cancel row — a ChoiceStep never aborts the operation
	// (Esc goes back to the previous step).
	Decide func(prev []Step) (apply bool, skipReason string, params NewSelectListParams)
	// Summary labels the choice in the completed-step summaries. Defaults to
	// SelectSummary (the chosen option's value).
	Summary func(model any) string
}

// ChoiceStep builds a wizard step that shows a SelectList of options only when it
// is relevant, deriving relevance, skip reason, and options from the earlier
// answers via Decide. It is the select analogue of ConfirmStep: an optional
// decision that always moves the flow forward (Esc = previous step), never
// cancelling — every option is an "apply". Decide runs once per entry in Build;
// its apply/reason results are cached and read back by AutoSkip and SkipReason in
// the same advance pass.
func ChoiceStep(p ChoiceStepParams) Step {
	summary := p.Summary
	if summary == nil {
		summary = SelectSummary
	}

	// Cached across Build → AutoSkip/SkipReason within a single advance() pass:
	// buildStep runs Build (setting applies/reason) immediately before AutoSkip.
	applies := true
	reason := ""
	build := func(prev []Step) any {
		apply, skipReason, params := p.Decide(prev)
		applies = apply
		reason = skipReason
		return NewSelectList(params)
	}

	return Step{
		Name:       p.Name,
		Model:      NewSelectList(NewSelectListParams{}),
		Build:      build,
		AutoSkip:   func(WizardModel) bool { return !applies },
		SkipReason: func() string { return reason },
		Summary:    summary,
	}
}
