package cli

import (
	"context"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// createForm is create over the ui/form engine, which asks every field of create's form by its kind.
type createForm = form.Command[create.Request, create.Questions, create.Outcome]

// createFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var createFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (create.UseCase, error)) *createForm {
	return &createForm{
		YesUsage: yesUsage,
		Open: func(ctx context.Context, dir string) (form.UseCase[create.Request, create.Questions, create.Outcome], error) {
			return open(ctx, dir)
		},
		Plan: func(previewed create.Outcome, width int, st *output.Styles) string {
			return planTextOf(previewed.Plan, previewed.Warnings, width, st)
		},
		ApplyLabel: applyLabel,
	}
}

// applyLabel counts the worktrees the plan previewed opens, a worktree already open aside.
func applyLabel(previewed create.Outcome) string {
	count := 0
	for _, planned := range previewed.Plan.Branches {
		if planned.Action != create.ActionUnchanged {
			count++
		}
	}

	switch count {
	case 0:
		return "Yes, apply"
	case 1:
		return "Yes, create worktree"
	}

	return "Yes, create " + strconv.Itoa(count) + " worktrees"
}
