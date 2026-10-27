package cli

import (
	"context"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/command/exec"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// execForm is exec over the ui/form engine, which asks every field of exec's form by its kind.
type execForm = form.Command[exec.Request, exec.Questions, exec.Outcome]

// execFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var execFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (exec.UseCase, error)) *execForm {
	return &execForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[exec.Request, exec.Questions, exec.Outcome](open),
		Preview:    form.PreviewOf[exec.Plan, exec.Detail],
		Draw:       drawOutcome,
		ApplyLabel: applyLabel,
		NewProgress: func(ctx context.Context, params output.ProgressParams) form.Progress {
			return newProgress(ctx, params)
		},
	}
}

// applyLabel names the apply of the plan previewed by the worktrees it runs in.
func applyLabel(previewed exec.Outcome) string {
	if targets := previewed.Plan.Targets; len(targets) == 1 {
		return "Yes, run in " + targets[0].Worktree
	}
	return "Yes, run in " + strconv.Itoa(len(previewed.Plan.Targets)) + " worktrees"
}
