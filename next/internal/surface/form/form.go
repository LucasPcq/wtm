// Package form is the interactive form of create: each question asked in turn, the plan recapped, then applied.
package form

import (
	"context"
	"errors"
	"io"

	"github.com/LucasPcq/wtm/next/internal/app"
	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	tea "github.com/charmbracelet/bubbletea"
)

const surface = "tui"

type App interface {
	CreateQuestions(ctx context.Context, req create.Request) (create.Questions, error)
	Create(ctx context.Context, params app.CreateParams) (app.CreateOutcome, error)
}

type Params struct {
	App     App
	Request create.Request
	DryRun  bool
	Report  wtm.Report
	In      io.Reader
	Out     io.Writer
}

type Ran struct {
	Outcome app.CreateOutcome
	Request create.Request
}

func Run(ctx context.Context, params Params) (Ran, error) {
	w := newWizard(ctx, wizardParams{App: params.App, Request: params.Request, DryRun: params.DryRun})
	for {
		answered, err := ask(ctx, askParams{Wizard: w, In: params.In, Out: params.Out})
		if err != nil {
			return Ran{Request: w.req}, err
		}
		create := app.CreateParams{Request: answered.req, DryRun: params.DryRun, Surface: surface, Report: params.Report}
		if !params.DryRun {
			create.ExpectFingerprint = answered.preview.Fingerprint
		}
		outcome, err := params.App.Create(ctx, create)
		if stale, ok := errors.AsType[*wtm.Error](err); !ok || stale.Code != app.CodePlanStale || params.DryRun {
			return Ran{Outcome: outcome, Request: answered.req}, err
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

func interrupted(cause error) *wtm.Error {
	return &wtm.Error{Kind: wtm.KindCancelled, Code: wtm.CodeInterrupted, Message: "interrupted", Cause: cause}
}
