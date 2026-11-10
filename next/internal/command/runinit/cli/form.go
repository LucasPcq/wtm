package cli

import (
	"context"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/command/runinit"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// runInitForm is run init over the ui/form engine, which asks every field by its kind, part after part.
type runInitForm = form.Command[runinit.Request, runinit.Questions, runinit.Outcome]

// runInitFlags spells each field path of the request as the flag that sets it, for the failures and the wizard.
var runInitFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (runinit.UseCase, error)) *runInitForm {
	return &runInitForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[runinit.Request, runinit.Questions, runinit.Outcome](open),
		Preview:    form.PreviewOf[runinit.Plan, runinit.Detail],
		Draw:       drawOutcome,
		ApplyLabel: applyLabel,
	}
}

// applyLabel names the write the plan previewed makes: its files.
func applyLabel(previewed runinit.Outcome) string {
	switch n := len(previewed.Plan.Files); n {
	case 0:
		return "Yes"
	case 1:
		return "Yes, write " + previewed.Plan.Files[0].Path
	default:
		return "Yes, write the " + strconv.Itoa(n) + " files"
	}
}
