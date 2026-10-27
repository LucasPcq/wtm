package cli

import (
	"context"
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/command/reparent"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// reparentForm is reparent over the ui/form engine, which asks every field of reparent's form by its kind.
type reparentForm = form.Command[reparent.Request, reparent.Questions, reparent.Outcome]

// reparentFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var reparentFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (reparent.UseCase, error)) *reparentForm {
	return &reparentForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[reparent.Request, reparent.Questions, reparent.Outcome](open),
		Preview:    form.PreviewOf[reparent.Plan, reparent.Detail],
		Draw:       drawOutcome,
		ApplyLabel: applyLabel,
	}
}

// applyLabel names the apply of the plan previewed by the worktrees it moves, those already on the new parent left out.
func applyLabel(previewed reparent.Outcome) string {
	moving := slices.DeleteFunc(slices.Clone(previewed.Plan.Moves), func(move reparent.Move) bool { return move.From == move.To })
	switch count := len(moving); count {
	case 0:
		return "Yes, apply"
	case 1:
		return "Yes, reparent " + moving[0].Branch
	default:
		return "Yes, reparent " + strconv.Itoa(count) + " worktrees"
	}
}
