package cli

import (
	"context"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// useCase is what the form calls of extract's use case, so a test can stand in for it.
type useCase interface {
	Questions(ctx context.Context, req extract.Request) (extract.Questions, error)
	Run(ctx context.Context, req extract.Request, opts domain.RunOptions) (extract.Outcome, error)
}

type (
	// extractForm is extract over the ui/form engine, which asks each field of extract's form by its kind but the three extract asks its own way (steps.go).
	extractForm = form.Command[useCase, extract.Request, extract.Questions, extract.Outcome]
	session     = form.Session[useCase, extract.Request, extract.Questions, extract.Outcome]
)

// extractFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var extractFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (extract.UseCase, error)) *extractForm {
	return &extractForm{
		YesUsage:   yesUsage,
		Open:       func(ctx context.Context, dir string) (useCase, error) { return open(ctx, dir) },
		Questions:  useCase.Questions,
		Run:        useCase.Run,
		Plan:       plan,
		ApplyLabel: applyLabel,
		Special:    special,
	}
}

func plan(previewed extract.Outcome, width int, st *output.Styles) string {
	return strings.Join(output.PlanLines(output.PlanParams{Rows: planRowsOf(previewed.Plan), Warnings: output.Messages(previewed.Warnings), Width: width, Styles: st}), "\n")
}

// applyLabel names the apply of the plan previewed: it creates the target first when the target does not exist.
func applyLabel(previewed extract.Outcome) string {
	switch previewed.Plan.Target.Action {
	case extract.ActionCreate, extract.ActionCheckOut, extract.ActionResume:
		return "Yes, create & extract"
	case extract.ActionExisting:
	}

	return "Yes, extract"
}
