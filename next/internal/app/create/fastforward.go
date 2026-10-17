package create

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

func (p ApplyParams) fastForward(ctx context.Context) []wtm.Warning {
	var warnings []wtm.Warning
	for _, subject := range p.Plan.FastForward {
		if ctx.Err() != nil {
			return warnings
		}
		p.Report(wtm.PhaseStarted{Subject: subject, Phase: "fast_forward"})
		warning, failed := p.advance(ctx, subject)
		p.Report(wtm.PhaseFinished{Subject: subject, Phase: "fast_forward", Failed: failed})
		if failed && ctx.Err() == nil {
			warnings = append(warnings, warning)
		}
	}
	return warnings
}

func (p ApplyParams) advance(ctx context.Context, subject string) (wtm.Warning, bool) {
	at := p.Facts.checkedOutAt(subject)
	if at == "" {
		return notAdvanced(subject, p.Deps.Git.FastForwardRef(ctx, subject))
	}
	if err := p.Deps.Git.FetchRef(ctx, subject); err != nil {
		return notAdvanced(subject, err)
	}
	dirty, err := p.Deps.Git.Dirty(ctx, at)
	if err != nil {
		return notAdvanced(subject, err)
	}
	if dirty {
		return wtm.Warning{
			Code:    "create.ff_dirty",
			Message: "couldn't fast-forward " + subject + ": its worktree has uncommitted changes — it starts from your local branch",
			Params:  wtm.Params{"branch": subject},
		}, true
	}
	return notAdvanced(subject, p.Deps.Git.MergeFFOnly(ctx, worktree.Merge{Path: at, Onto: branch.RemoteBranch{Name: subject}}))
}

func notAdvanced(subject string, err error) (wtm.Warning, bool) {
	if err == nil {
		return wtm.Warning{}, false
	}
	return wtm.Warning{
		Code:    "create.ff_failed",
		Message: "couldn't fast-forward " + subject + ": " + err.Error() + " — it starts from your local branch",
		Params:  wtm.Params{"branch": subject, "cause": err.Error()},
	}, true
}
