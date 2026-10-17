package cli

import (
	"context"
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// useCase is what the wizard calls of extract's use case, so a test can stand in for it.
type useCase interface {
	Questions(ctx context.Context, req extract.Request) (extract.Questions, error)
	Run(ctx context.Context, req extract.Request, opts domain.RunOptions) (extract.Outcome, error)
}

type formParams struct {
	App     useCase
	Request extract.Request
	DryRun  bool
	Report  domain.Report
	In      io.Reader
	Out     *output.Stream
}

// ran is how a run went and the request it ran, which the wizard's answers may have changed.
type ran struct {
	Outcome extract.Outcome
	Request extract.Request
}

// runForm runs the wizard on the terminal through to the apply; it returns the request as answered and its outcome, with form.Run's error: a cancel, a failed read of the repository, or the refusal of the run.
func runForm(ctx context.Context, params formParams) (ran, error) {
	w := newWizard(ctx, wizardParams{App: params.App, Request: params.Request, DryRun: params.DryRun, Report: params.Report})
	err := form.Run(ctx, w.form(), params.In, params.Out)

	return ran{Outcome: w.outcome, Request: w.req}, err
}
