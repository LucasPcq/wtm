package cli

import (
	"context"
	"strconv"

	envcmd "github.com/LucasPcq/wtm/next/internal/command/env"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// envForm is env over the ui/form engine, which asks every field of its form by its kind but the worktrees, which its own step asks.
type envForm = form.Command[envcmd.Request, envcmd.Questions, envcmd.Outcome]

// envFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var envFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (envcmd.UseCase, error)) *envForm {
	return &envForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[envcmd.Request, envcmd.Questions, envcmd.Outcome](open),
		Preview:    form.PreviewOf[envcmd.Plan, envcmd.Detail],
		Draw:       drawOutcome,
		ApplyLabel: applyLabel,
		Special:    special,
		Skipped:    skipped,
	}
}

// applyLabel names the apply of the plan previewed, which changes something (an empty one is not asked to be confirmed), by the worktrees it writes.
func applyLabel(previewed envcmd.Outcome) string {
	var writing []string
	for _, r := range previewed.Plan.Worktrees {
		if !r.Missing && changes(r) {
			writing = append(writing, r.Worktree)
		}
	}

	if len(writing) == 1 {
		return "Yes, reconcile " + writing[0]
	}
	return "Yes, reconcile " + strconv.Itoa(len(writing)) + " worktrees"
}

// changes reports whether the recap of r's plan says it writes something.
func changes(r envcmd.Reconcile) bool {
	if !r.Numbered || r.Records != "" {
		return true
	}
	for _, file := range r.Files {
		if file.Writes {
			return true
		}
	}
	return false
}

// skipped words why a field of env's is not asked, where the wizard's words would say it of another command.
func skipped(_ *session, skip domain.Skip) string {
	if skip == domain.SkipAll {
		return "every worktree is asked"
	}
	return ""
}
