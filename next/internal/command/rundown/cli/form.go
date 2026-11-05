package cli

import (
	"context"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/command/rundown"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// downForm is run down over the ui/form engine, which asks every field of run down's form by its kind.
type downForm = form.Command[rundown.Request, rundown.Questions, rundown.Outcome]

type session = form.Session[rundown.Request, rundown.Questions, rundown.Outcome]

// downFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var downFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (rundown.UseCase, error)) *downForm {
	return &downForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[rundown.Request, rundown.Questions, rundown.Outcome](open),
		Preview:    form.PreviewOf[rundown.Plan, rundown.Detail],
		Draw:       drawOutcome,
		ApplyLabel: applyLabel,
		Skipped:    skipped,
		NewProgress: func(ctx context.Context, params output.ProgressParams) form.Progress {
			return output.NewConcurrent(ctx, output.ConcurrentParams{ProgressParams: params, Rows: output.RowPerJob})
		},
	}
}

// applyLabel names the apply of the plan previewed by the worktrees it stops the jobs of.
func applyLabel(previewed rundown.Outcome) string {
	if worktrees := previewed.Plan.Worktrees; len(worktrees) == 1 {
		return "Yes, stop the jobs of " + worktrees[0].Worktree
	}
	return "Yes, stop the jobs of " + strconv.Itoa(len(previewed.Plan.Worktrees)) + " worktrees"
}

// skipped words why a step of run down is not asked, where the wizard's own words would mislead.
func skipped(_ *session, skip domain.Skip) string {
	if skip == domain.SkipAll {
		return "--all stops every worktree's jobs"
	}
	return ""
}
