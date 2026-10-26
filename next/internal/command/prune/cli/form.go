package cli

import (
	"context"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/command/prune"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// pruneForm is prune over the ui/form engine, which asks every field of prune's form by its kind.
type pruneForm = form.Command[prune.Request, prune.Questions, prune.Outcome]

// pruneFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var pruneFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (prune.UseCase, error)) *pruneForm {
	return &pruneForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[prune.Request, prune.Questions, prune.Outcome](open),
		Preview:    form.PreviewOf[prune.Plan, prune.Detail],
		Draw:       drawOutcome,
		ApplyLabel: applyLabel,
		Special:    special,
	}
}

// applyLabel names the apply of the plan previewed by the worktrees it prunes.
func applyLabel(previewed prune.Outcome) string {
	switch count := len(previewed.Plan.Worktrees); count {
	case 0:
		return "Yes, apply"
	case 1:
		return "Yes, prune " + previewed.Plan.Worktrees[0].Worktree
	default:
		return "Yes, prune " + strconv.Itoa(count) + " worktrees"
	}
}
