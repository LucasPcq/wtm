package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/initialize"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// initForm is init over the ui/form engine, which asks every setting by its kind.
type initForm = form.Command[initialize.Request, initialize.Questions, initialize.Outcome]

// initFlags spells each field path of the request as the flag that sets it, for the failures and the wizard.
var initFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (initialize.UseCase, error)) *initForm {
	return &initForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[initialize.Request, initialize.Questions, initialize.Outcome](open),
		Preview:    form.PreviewOf[initialize.Plan, initialize.Detail],
		Draw:       drawOutcome,
		ApplyLabel: applyLabel,
	}
}

// applyLabel names the write the plan previewed makes: a first set-up, or the settings set up again.
func applyLabel(previewed initialize.Outcome) string {
	if previewed.Plan.Created {
		return "Yes, set it up"
	}

	return "Yes, save the settings"
}
