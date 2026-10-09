package components

// ConfirmStepParams configures a conditional confirmation step built by
// ConfirmStep.
type ConfirmStepParams struct {
	// Name labels the step in the breadcrumb and completed-step summaries.
	Name string
	// Decide runs each time the wizard enters this step. It inspects the
	// already-completed prior steps and returns whether the confirmation is
	// needed, the reason to show when it is not (rendered as "⊘ <Name> — <reason>"
	// in the summaries), and the prompt to show when it is. When apply is false the
	// step is auto-skipped.
	Decide func(prev []Step) (apply bool, skipReason string, params NewConfirmParams)
	// YesLabel and NoLabel label the choice in the completed-step summary.
	// They default to "yes"/"no".
	YesLabel string
	NoLabel  string
	// Callout renders the question's description as an emphasized intro, the way
	// the list steps do. Without it a confirmation reads as muted aside next to
	// its neighbours.
	Callout bool
}

// ConfirmStep builds a wizard step that shows a yes/no confirmation only when it
// is relevant, deriving both its relevance and its wording from the earlier
// answers via Decide. It packages the Build + AutoSkip pair the sync/relocate
// wizards use, so a confirmation lives inside the wizard — with breadcrumb and
// back navigation — instead of as an orphaned standalone prompt after it (where
// Esc would abort the whole operation). Decide runs once per entry in Build; its
// applies result is cached and read back by AutoSkip in the same advance pass.
func ConfirmStep(p ConfirmStepParams) Step {
	yes, no := p.YesLabel, p.NoLabel
	if yes == "" {
		yes = "yes"
	}
	if no == "" {
		no = "no"
	}

	// Cached across Build → AutoSkip/SkipReason within a single advance() pass:
	// buildStep runs Build (setting applies/reason) immediately before AutoSkip.
	applies := true
	reason := ""
	build := func(prev []Step) any {
		apply, skipReason, params := p.Decide(prev)
		applies = apply
		reason = skipReason
		return NewConfirm(params)
	}

	return Step{
		Name:       p.Name,
		Model:      NewConfirm(NewConfirmParams{}),
		Build:      build,
		AutoSkip:   func(WizardModel) bool { return !applies },
		SkipReason: func() string { return reason },
		Summary:    ConfirmSummary(yes, no),
		Callout:    p.Callout,
	}
}
