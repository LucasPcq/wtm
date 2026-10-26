package cli

import (
	"context"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/command/clean"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// cleanForm is clean over the ui/form engine, which asks every field of clean's form by its kind.
type cleanForm = form.Command[clean.Request, clean.Questions, clean.Outcome]

// cleanFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var cleanFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (clean.UseCase, error)) *cleanForm {
	return &cleanForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[clean.Request, clean.Questions, clean.Outcome](open),
		Preview:    form.PreviewOf[clean.Plan, clean.Detail],
		Draw:       drawOutcome,
		ApplyLabel: applyLabel,
	}
}

// applyLabel names the apply of the plan previewed by the worktrees it removes.
func applyLabel(previewed clean.Outcome) string {
	switch count := len(previewed.Plan.Worktrees); count {
	case 0:
		return "Yes, apply"
	case 1:
		return "Yes, remove " + previewed.Plan.Worktrees[0].Worktree
	default:
		return "Yes, remove " + strconv.Itoa(count) + " worktrees"
	}
}
