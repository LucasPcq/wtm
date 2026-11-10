package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstop"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// stopForm is daemon stop over the ui/form engine.
type stopForm = form.Command[daemonstop.Request, daemonstop.Questions, daemonstop.Outcome]

// stopFlags spells each field path of the request as its flag: daemon stop has none.
var stopFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (daemonstop.UseCase, error)) *stopForm {
	return &stopForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[daemonstop.Request, daemonstop.Questions, daemonstop.Outcome](open),
		Preview:    form.PreviewOf[daemonstop.Plan, daemonstop.Detail],
		Draw:       drawOutcome,
		ApplyLabel: func(daemonstop.Outcome) string { return "Yes, stop wtm's daemon" },
		Unasked:    func(previewed daemonstop.Outcome) bool { return !previewed.Plan.Confirm },
	}
}
