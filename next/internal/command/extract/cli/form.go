package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

type (
	// extractForm is extract over the ui/form engine, which asks each field of extract's form by its kind but the three extract asks its own way (steps.go).
	extractForm = form.Command[extract.Request, extract.Questions, extract.Outcome]
	session     = form.Session[extract.Request, extract.Questions, extract.Outcome]
)

// extractFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var extractFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (extract.UseCase, error)) *extractForm {
	return &extractForm{
		YesUsage:   yesUsage,
		Open:       form.Opener[extract.Request, extract.Questions, extract.Outcome](open),
		Preview:    preview,
		Draw:       drawOutcome,
		ApplyLabel: applyLabel,
		Special:    special,
	}
}

// applyLabel names the apply of the plan previewed, which creates the target first when the plan says so.
func applyLabel(previewed extract.Outcome) string {
	if previewed.Plan.Creates > 0 {
		return "Yes, create & extract"
	}
	return "Yes, extract"
}
