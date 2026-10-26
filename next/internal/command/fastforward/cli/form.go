package cli

import (
	"context"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// fastForwardForm is fast-forward over the ui/form engine, which asks every field of its form by its kind but the worktrees, which its own step asks.
type fastForwardForm = form.Command[fastforward.Request, fastforward.Questions, fastforward.Outcome]

// fastForwardFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var fastForwardFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (fastforward.UseCase, error)) *fastForwardForm {
	return &fastForwardForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[fastforward.Request, fastforward.Questions, fastforward.Outcome](open),
		Preview:    form.PreviewOf[fastforward.Plan, fastforward.Detail],
		Draw:       drawOutcome,
		ApplyLabel: applyLabel,
		Special:    special,
		Skipped:    skipped,
	}
}

// applyLabel names the apply of the plan previewed by the worktrees it moves.
func applyLabel(previewed fastforward.Outcome) string {
	var moving []string
	for _, a := range previewed.Plan.Worktrees {
		if len(a.Phases) > 0 {
			moving = append(moving, a.Worktree)
		}
	}

	switch len(moving) {
	case 0:
		return "Yes, apply"
	case 1:
		return "Yes, fast-forward " + moving[0]
	default:
		return "Yes, fast-forward " + strconv.Itoa(len(moving)) + " worktrees"
	}
}

// skipped words why a field of fast-forward's is not asked, where the wizard's words would say it of another command.
func skipped(s *session, skip domain.Skip) string {
	switch {
	case skip == domain.SkipAll:
		return "every worktree is asked"
	case skip == domain.SkipFlagOnly && s.Request().All:
		return "every worktree: --all, or the default pick confirmed as it was"
	case skip == domain.SkipFlagOnly:
		return "only --all asks every worktree"
	case skip == domain.SkipSafe:
		return "no worktree named behind origin has uncommitted changes"
	}
	return ""
}
