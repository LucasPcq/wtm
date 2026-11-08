package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/daemonrestart"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// restartForm is daemon restart over the ui/form engine.
type restartForm = form.Command[daemonrestart.Request, daemonrestart.Questions, daemonrestart.Outcome]

// restartFlags spells each field path of the request as its flag: daemon restart has none.
var restartFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (daemonrestart.UseCase, error)) *restartForm {
	return &restartForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[daemonrestart.Request, daemonrestart.Questions, daemonrestart.Outcome](open),
		Preview:    form.PreviewOf[daemonrestart.Plan, daemonrestart.Detail],
		Draw:       drawOutcome,
		ApplyLabel: func(daemonrestart.Outcome) string { return "Yes, restart wtm's daemon" },
		Unasked:    func(previewed daemonrestart.Outcome) bool { return !previewed.Plan.Confirm },
	}
}
