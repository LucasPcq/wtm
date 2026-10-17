package app

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
)

type CreateParams struct {
	Request           create.Request
	DryRun            bool
	ExpectFingerprint string
	Surface           string
	Report            wtm.Report
}

type CreateOutcome = Outcome[create.Plan, create.Result]

func (a *App) Create(ctx context.Context, params CreateParams) (CreateOutcome, error) {
	return run(ctx, a, steps[create.Plan, create.Result]{
		Command: "create",
		Locks:   params.Request.Locks(),
		Plan: func(ctx context.Context) (create.Plan, []wtm.Warning, error) {
			return a.deps.Create.Plan(ctx, params.Request)
		},
		Apply: a.deps.Create.Apply,
	}, Params{DryRun: params.DryRun, ExpectFingerprint: params.ExpectFingerprint, Surface: params.Surface, Report: params.Report})
}

func (a *App) CreateQuestions(ctx context.Context, req create.Request) (create.Questions, error) {
	return a.deps.Create.Questions(ctx, req)
}
