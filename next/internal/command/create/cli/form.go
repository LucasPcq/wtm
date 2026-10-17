package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// useCase is what the form calls of create's use case, so a test can stand in for it.
type useCase interface {
	Questions(ctx context.Context, req create.Request) (create.Questions, error)
	Run(ctx context.Context, req create.Request, opts domain.RunOptions) (create.Outcome, error)
}

// createForm is create over the ui/form engine, which asks every field of create's form by its kind.
type createForm = form.Command[useCase, create.Request, create.Questions, create.Outcome]

// createFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var createFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (create.UseCase, error)) *createForm {
	return &createForm{
		YesUsage:  yesUsage,
		Open:      func(ctx context.Context, dir string) (useCase, error) { return open(ctx, dir) },
		Questions: useCase.Questions,
		Run:       useCase.Run,
		Plan: func(previewed create.Outcome, width int, st *output.Styles) string {
			return planTextOf(previewed.Plan, previewed.Warnings, width, st)
		},
	}
}
