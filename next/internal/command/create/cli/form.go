package cli

import (
	"context"
	"errors"
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	tea "github.com/charmbracelet/bubbletea"
)

const surfaceTUI = "tui"

type useCase interface {
	Questions(ctx context.Context, req create.Request) (create.Questions, error)
	Run(ctx context.Context, req create.Request, opts operation.Options) (create.Outcome, error)
}

type formParams struct {
	App     useCase
	Request create.Request
	DryRun  bool
	Report  domain.Report
	In      io.Reader
	Out     io.Writer
}

type ran struct {
	Outcome create.Outcome
	Request create.Request
}

func runForm(ctx context.Context, params formParams) (ran, error) {
	w := newWizard(ctx, wizardParams{App: params.App, Request: params.Request, DryRun: params.DryRun})
	if params.DryRun {
		if err := w.reobserve(); err != nil {
			return ran{Request: params.Request}, err
		}
		if !w.asks() {
			outcome, err := params.App.Run(ctx, params.Request, operation.Options{DryRun: true, Surface: surfaceTUI})
			return ran{Outcome: outcome, Request: params.Request}, err
		}
	}
	for {
		answered, err := ask(ctx, askParams{Wizard: w, In: params.In, Out: params.Out})
		if err != nil {
			return ran{Request: w.req}, err
		}
		opts := operation.Options{DryRun: params.DryRun, Surface: surfaceTUI, Report: params.Report}
		if !params.DryRun {
			opts.ExpectFingerprint = answered.preview.Fingerprint
		}
		outcome, err := params.App.Run(ctx, answered.req, opts)
		if stale, ok := errors.AsType[*domain.Error](err); !ok || stale.Code != operation.CodePlanStale || params.DryRun {
			return ran{Outcome: outcome, Request: answered.req}, err
		}
		w = answered.again()
	}
}

type askParams struct {
	Wizard *wizard
	In     io.Reader
	Out    io.Writer
}

func ask(ctx context.Context, params askParams) (*wizard, error) {
	final, err := tea.NewProgram(params.Wizard, tea.WithContext(ctx), tea.WithInput(params.In), tea.WithOutput(params.Out), tea.WithoutSignalHandler()).Run()
	if err != nil {
		return nil, interrupted(err)
	}
	w, ok := final.(*wizard)
	switch {
	case !ok:
		return nil, interrupted(nil)
	case w.err != nil:
		return nil, w.err
	case w.aborted:
		return nil, interrupted(nil)
	}
	return w, nil
}

func interrupted(cause error) *domain.Error {
	return &domain.Error{Kind: domain.KindCancelled, Code: domain.CodeInterrupted, Message: "interrupted", Cause: cause}
}
