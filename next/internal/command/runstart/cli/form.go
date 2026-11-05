package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/runstart"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// startForm is run start over the ui/form engine, which asks every field of run start's form by its kind.
type startForm = form.Command[runstart.Request, runstart.Questions, runstart.Outcome]

// session is a wizard over run start's form.
type session = form.Session[runstart.Request, runstart.Questions, runstart.Outcome]

// startFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var startFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (runstart.UseCase, error)) *startForm {
	return &startForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[runstart.Request, runstart.Questions, runstart.Outcome](open),
		Preview:    form.PreviewOf[runstart.Plan, runstart.Detail],
		Draw:       drawOutcome,
		ApplyLabel: applyLabel,
		Skipped:    skipped,
		NewProgress: func(ctx context.Context, params output.ProgressParams) form.Progress {
			return output.NewConcurrent(ctx, output.ConcurrentParams{ProgressParams: params, Rows: output.RowPerJob})
		},
	}
}

// applyLabel names the apply of the plan previewed by its job, what the start does to it, and its worktree.
func applyLabel(previewed runstart.Outcome) string {
	verb := "start "
	if previewed.Plan.Restarts() {
		verb = "start again "
	}
	in := ""
	if worktrees := previewed.Plan.Worktrees; len(worktrees) == 1 {
		in = " in " + worktrees[0].Worktree
	}
	return "Yes, " + verb + previewed.Plan.Job + in
}

// skipped words why a step of run start is not asked, where the wizard's own words would mislead.
func skipped(_ *session, skip domain.Skip) string {
	if skip == domain.SkipSafe {
		return "the job changes no data its worktree does not own"
	}
	return ""
}
