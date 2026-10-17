package create

import (
	"context"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

func (f Facts) subjects(req Request) []string {
	var subjects []string
	for _, name := range req.Branches {
		subject := ""
		switch {
		case f.existing(name):
			subject = name
		case !f.local(name) && !f.remote(name) && f.local(req.From):
			subject = req.From
		}
		if subject != "" && !slices.Contains(subjects, subject) {
			subjects = append(subjects, subject)
		}
	}
	return subjects
}

func (f Facts) behind(req Request) []string {
	return slices.DeleteFunc(f.subjects(req), func(subject string) bool { return f.standing(subject) != branch.Behind })
}

func (f Facts) diverged(req Request) []string {
	return slices.DeleteFunc(f.subjects(req), func(subject string) bool { return f.standing(subject) != branch.Diverged })
}

func (a applying) fastForward(ctx context.Context) []wtm.Warning {
	var warnings []wtm.Warning
	for _, subject := range a.Plan.FastForward {
		if ctx.Err() != nil {
			return warnings
		}
		a.Report(wtm.PhaseStarted{Subject: subject, Phase: "fast_forward"})
		warning, failed := a.advance(ctx, subject)
		a.Report(wtm.PhaseFinished{Subject: subject, Phase: "fast_forward", Failed: failed})
		if failed && ctx.Err() == nil {
			warnings = append(warnings, warning)
		}
	}
	return warnings
}

func (a applying) advance(ctx context.Context, subject string) (wtm.Warning, bool) {
	at := a.Facts.checkedOutAt(subject)
	if at == "" {
		return notAdvanced(subject, a.Deps.Git.FastForwardRef(ctx, subject))
	}
	if err := a.Deps.Git.FetchRef(ctx, subject); err != nil {
		return notAdvanced(subject, err)
	}
	dirty, err := a.Deps.Git.Dirty(ctx, at)
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
	return notAdvanced(subject, a.Deps.Git.MergeFFOnly(ctx, worktree.Merge{Path: at, Onto: branch.RemoteBranch{Name: subject}}))
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
