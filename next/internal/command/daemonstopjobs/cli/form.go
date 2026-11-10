package cli

import (
	"context"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstopjobs"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// stopJobsForm is daemon stop-jobs over the ui/form engine.
type stopJobsForm = form.Command[daemonstopjobs.Request, daemonstopjobs.Questions, daemonstopjobs.Outcome]

// stopJobsFlags spells each field path of the request as its flag.
var stopJobsFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (daemonstopjobs.UseCase, error)) *stopJobsForm {
	return &stopJobsForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[daemonstopjobs.Request, daemonstopjobs.Questions, daemonstopjobs.Outcome](open),
		Preview:    form.PreviewOf[daemonstopjobs.Plan, daemonstopjobs.Detail],
		Draw:       drawOutcome,
		ApplyLabel: applyLabel,
		NewProgress: func(ctx context.Context, params output.ProgressParams) form.Progress {
			return output.NewConcurrent(ctx, output.ConcurrentParams{ProgressParams: params, Rows: output.RowPerJob})
		},
	}
}

// applyLabel names the apply of the plan previewed by how many jobs it stops.
func applyLabel(previewed daemonstopjobs.Outcome) string {
	n := 0
	for _, repo := range previewed.Plan.Repos {
		for _, t := range repo.Worktrees {
			n += len(t.Jobs)
		}
	}
	if n == 1 {
		return "Yes, stop 1 job"
	}
	return "Yes, stop " + strconv.Itoa(n) + " jobs"
}
