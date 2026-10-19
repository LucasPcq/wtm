package cli

import (
	"context"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// createForm is create over the ui/form engine, which asks every field of create's form by its kind.
type createForm = form.Command[create.Request, create.Questions, create.Outcome]

// createFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var createFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (create.UseCase, error)) *createForm {
	return &createForm{
		YesUsage:   yesUsage,
		Open:       form.Opener[create.Request, create.Questions, create.Outcome](open),
		Preview:    preview,
		Draw:       drawOutcome,
		ApplyLabel: applyLabel,
	}
}

// applyLabel names the apply of the plan previewed by the worktrees it opens.
func applyLabel(previewed create.Outcome) string {
	count := previewed.Plan.Creates
	switch count {
	case 0:
		return "Yes, apply"
	case 1:
		return "Yes, create worktree"
	}

	return "Yes, create " + strconv.Itoa(count) + " worktrees"
}
