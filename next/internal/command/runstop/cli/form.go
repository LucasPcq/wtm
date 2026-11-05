package cli

import (
	"context"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/command/runstop"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// stopForm is run stop over the ui/form engine, which asks every field of run stop's form by its kind.
type stopForm = form.Command[runstop.Request, runstop.Questions, runstop.Outcome]

// stopFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var stopFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (runstop.UseCase, error)) *stopForm {
	return &stopForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[runstop.Request, runstop.Questions, runstop.Outcome](open),
		Preview:    form.PreviewOf[runstop.Plan, runstop.Detail],
		Draw:       drawOutcome,
		ApplyLabel: applyLabel,
		NewProgress: func(ctx context.Context, params output.ProgressParams) form.Progress {
			return output.NewConcurrent(ctx, output.ConcurrentParams{ProgressParams: params, Rows: output.RowPerJob})
		},
	}
}

// applyLabel names the apply of the plan previewed by its job and the worktrees it stops it in.
func applyLabel(previewed runstop.Outcome) string {
	if worktrees := previewed.Plan.Worktrees; len(worktrees) == 1 {
		return "Yes, stop " + previewed.Plan.Job + " in " + worktrees[0].Worktree
	}
	return "Yes, stop " + previewed.Plan.Job + " in " + strconv.Itoa(len(previewed.Plan.Worktrees)) + " worktrees"
}
