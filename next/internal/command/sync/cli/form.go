package cli

import (
	"context"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/command/sync"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// syncForm is sync over the ui/form engine, which asks every field of its form by its kind but the worktrees, which its own step asks.
type syncForm = form.Command[sync.Request, sync.Questions, sync.Outcome]

// syncFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
var syncFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (sync.UseCase, error)) *syncForm {
	return &syncForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[sync.Request, sync.Questions, sync.Outcome](open),
		Preview:    form.PreviewOf[sync.Plan, sync.Detail],
		Draw:       drawOutcome,
		ApplyLabel: applyLabel,
		Special:    special,
		Skipped:    skipped,
	}
}

// applyLabel names the apply of the plan previewed, which changes something (an empty one is not asked to be confirmed), by the worktrees it syncs, or else by the branches it fast-forwards.
func applyLabel(previewed sync.Outcome) string {
	var moving []string
	for _, r := range previewed.Plan.Worktrees {
		if len(r.Phases) > 0 {
			moving = append(moving, r.Worktree)
		}
	}
	switch len(moving) {
	case 0:
		return "Yes, fast-forward the parents"
	case 1:
		return "Yes, sync " + moving[0]
	}
	return "Yes, sync " + strconv.Itoa(len(moving)) + " worktrees"
}

// skipped words why a field of sync's is not asked, where the wizard's words would say it of another command.
func skipped(_ *session, skip domain.Skip) string {
	switch skip {
	case domain.SkipAll:
		return "every worktree is asked"
	case domain.SkipSafe:
		return "no worktree named has uncommitted changes"
	case domain.SkipNotBehind:
		return "no branch rebased onto is on origin"
	case domain.SkipNothingToPush:
		return "no worktree that may rebase has its branch on origin"
	}
	return ""
}
