package provision

import (
	"context"
	"fmt"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// FastForwardParams names the branch to fast-forward to its origin.
type FastForwardParams struct {
	Subject      string // the local branch
	CheckedOutAt string // the worktree it is checked out in; empty when it is in none
}

// FastForwarded is what FastForward did: Dirty when the branch's worktree had
// changes and was left alone, Busy when it was stopped in a rebase or a bisect
// and was left alone, Err when git failed.
type FastForwarded struct {
	Dirty bool
	Busy  bool
	Err   error
}

// Failed reports whether the branch was left where it was.
func (f FastForwarded) Failed() bool {
	return f.Dirty || f.Busy || f.Err != nil
}

// FastForwardEachParams names the branches to fast-forward and how the command
// that asks words the one it cannot move.
type FastForwardEachParams struct {
	Command  string            // the command whose codes the warnings carry: <command>.ff_dirty (worktree dirty), .ff_busy (mid-rebase or bisect), .ff_failed (git failed, its cause in params)
	Subjects []string          // the local branches, in order
	Snapshot worktree.Snapshot // where each subject is checked out
	Fallback string            // what starts from a subject left behind, ending its warning: "it starts from your local branch"
}

// FastForwardEach fast-forwards each subject in turn, between its
// fast_forward phase events sent to report, starting none once ctx is
// cancelled. It returns the subjects it advanced, and a warning for each one
// left where it was, except one cut short by a cancel: the run reports the
// interruption instead.
func (s Service) FastForwardEach(ctx context.Context, params FastForwardEachParams, report domain.Report) ([]string, []domain.Warning) {
	var advanced []string
	var warnings []domain.Warning
	for _, subject := range params.Subjects {
		if ctx.Err() != nil {
			return advanced, warnings
		}

		report(domain.PhaseStarted{Subject: subject, Phase: "fast_forward"})
		forwarded := s.FastForward(ctx, FastForwardParams{Subject: subject, CheckedOutAt: params.Snapshot.CheckedOutAt(subject)})
		report(domain.PhaseFinished{Subject: subject, Phase: "fast_forward", Failed: forwarded.Failed()})

		switch {
		case !forwarded.Failed():
			advanced = append(advanced, subject)
		case ctx.Err() == nil:
			warnings = append(warnings, params.notAdvanced(subject, forwarded))
		}
	}

	return advanced, warnings
}

func (p FastForwardEachParams) notAdvanced(subject string, forwarded FastForwarded) domain.Warning {
	prefix, suffix := "couldn't fast-forward "+subject+": ", " — "+p.Fallback
	switch {
	case forwarded.Dirty:
		return domain.Warning{Code: p.Command + ".ff_dirty", Message: prefix + "its worktree has uncommitted changes" + suffix, Params: domain.Params{"branch": subject}}
	case forwarded.Busy:
		return domain.Warning{Code: p.Command + ".ff_busy", Message: prefix + "its worktree is in the middle of a rebase or a bisect" + suffix, Params: domain.Params{"branch": subject}}
	}

	cause := forwarded.Err.Error()
	return domain.Warning{Code: p.Command + ".ff_failed", Message: prefix + cause + suffix, Params: domain.Params{"branch": subject, "cause": cause}}
}

// FastForward brings params.Subject up to its origin branch, never merging:
// the ref alone when no worktree has it checked out, else a fast-forward merge
// in that worktree, which it skips when the worktree is not plainly on the
// branch (mid-rebase or bisect: the merge would move its detached HEAD, not
// the branch) or is dirty.
func (s Service) FastForward(ctx context.Context, params FastForwardParams) FastForwarded {
	if params.CheckedOutAt == "" {
		return FastForwarded{Err: s.Git.FastForwardRef(ctx, params.Subject)}
	}

	if err := s.Git.FetchRef(ctx, params.Subject); err != nil {
		return FastForwarded{Err: err}
	}

	busy, err := s.busy(ctx, params.CheckedOutAt)
	if err != nil || busy {
		return FastForwarded{Busy: busy, Err: err}
	}

	dirty, err := s.Git.Dirty(ctx, params.CheckedOutAt)
	if err != nil || dirty {
		return FastForwarded{Dirty: dirty, Err: err}
	}

	return FastForwarded{Err: s.Git.MergeFFOnly(ctx, worktree.Merge{Path: params.CheckedOutAt, Onto: worktree.RemoteBranch{Name: params.Subject}})}
}

// busy reports whether the worktree at path has its branch in an operation
// rather than checked out: any state but OnBranch.
func (s Service) busy(ctx context.Context, path string) (bool, error) {
	worktrees, err := s.Git.Worktrees(ctx)
	if err != nil {
		return false, fmt.Errorf("list worktrees: %w", err)
	}

	for _, listed := range worktrees {
		if listed.Path == path {
			_, onBranch := listed.State.(worktree.OnBranch)
			return !onBranch, nil
		}
	}
	return false, nil
}
