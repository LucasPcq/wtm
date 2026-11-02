package cli

import (
	"context"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/command/runup"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// upForm is run up over the ui/form engine, which asks every field of run up's form by its kind.
type upForm = form.Command[runup.Request, runup.Questions, runup.Outcome]

// session is a wizard over run up's form.
type session = form.Session[runup.Request, runup.Questions, runup.Outcome]

// upFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var upFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (runup.UseCase, error)) *upForm {
	return &upForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[runup.Request, runup.Questions, runup.Outcome](open),
		Preview:    form.PreviewOf[runup.Plan, runup.Detail],
		Draw:       drawOutcome,
		ApplyLabel: applyLabel,
		Skipped:    skipped,
		NewProgress: func(ctx context.Context, params output.ProgressParams) form.Progress {
			return output.NewConcurrent(ctx, output.ConcurrentParams{ProgressParams: params, Rows: output.RowPerJob})
		},
	}
}

// applyLabel names the apply of the plan previewed by the profile it starts and the worktrees it starts it in.
func applyLabel(previewed runup.Outcome) string {
	what := "the jobs"
	if previewed.Plan.Profile != "" {
		what = previewed.Plan.Profile
	}
	if worktrees := previewed.Plan.Worktrees; len(worktrees) == 1 {
		return "Yes, start " + what + " in " + worktrees[0].Worktree
	}
	return "Yes, start " + what + " in " + strconv.Itoa(len(previewed.Plan.Worktrees)) + " worktrees"
}

// skipped words why a step of run up is not asked, where the wizard's own words would mislead.
func skipped(_ *session, skip domain.Skip) string {
	if skip == domain.SkipSafe {
		return "no job changes data its worktree does not own"
	}
	return ""
}
