package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/checkout"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// checkoutForm is checkout over the ui/form engine, which asks every field of checkout's form by its kind.
type checkoutForm = form.Command[checkout.Request, checkout.Questions, checkout.Outcome]

// checkoutFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var checkoutFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (checkout.UseCase, error)) *checkoutForm {
	return &checkoutForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[checkout.Request, checkout.Questions, checkout.Outcome](open),
		Preview:    form.PreviewOf[checkout.Plan, checkout.Detail],
		Draw:       drawOutcome,
		ApplyLabel: func(checkout.Outcome) string { return "Yes, check out" },
		Skipped:    skipped,
	}
}

// skipped words the fast-forward not asked of the pull request's own branch, where create's words speak of a source.
func skipped(_ *form.Session[checkout.Request, checkout.Questions, checkout.Outcome], skip domain.Skip) string {
	if skip == domain.SkipNotBehind {
		return "nothing to fast-forward: the branch is not a local one behind origin"
	}

	return ""
}
