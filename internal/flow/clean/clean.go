// Package clean runs the `wtm clean` flow.
package clean

import (
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/orphans"
	"github.com/LucasPcq/wtm/internal/flow/run/owed"
	"github.com/LucasPcq/wtm/internal/flow/teardown"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Request struct {
	Branches []string
	// Force is the safety axis: it lifts the locked/dirty/unpushed/open-PR refusals.
	Force            bool
	ReparentChildren bool
	BaseBranch       string
	// AllowPrivileged lets a failed removal offer the `sudo rm -rf` fallback. The
	// prompt it opens belongs to sudo and takes the terminal, so only a surface
	// that can hand it over sets this.
	AllowPrivileged bool
	// KeepData withholds the namespaces the worktrees carved out of shared
	// services. The default is to give them back: clean is the destructive
	// command, and removing a worktree without its data would leave an orphan
	// database behind on every iteration.
	KeepData bool
	// DropData drops it now instead, starting the services that are down.
	DropData bool
}

type Outcome struct {
	Results    []domain.CleanResult
	Failed     []domain.BatchFailure
	Skipped    []domain.PruneSkip
	Reparented []domain.ReparentResult
	Orphaned   []domain.ReparentResult
	Namespaces []domain.NamespaceOutcome
	Aborted    bool
}

// Presenter hears about each worktree only when the run removes several: a
// single one reads exactly as it always did.
type Presenter interface {
	flow.Presenter
	WorktreeStarted(flow.Progress)
	WorktreeCleaned(domain.CleanResult)
	WorktreeFailed(domain.BatchFailure)
	Cleaned(Outcome) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

// Operation declares how a surface must schedule a clean: it destroys its
// targets, so it holds the surface until it is done rather than running behind
// its back.
func Operation() flow.Operation {
	return flow.Operation{Kind: domain.OpKindClean, Mode: flow.ModeBlocking, TargetKey: KeyWorktree}
}

func Run(params Params) (Outcome, error) {
	f := &cleanFlow{
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
		checks:    make(map[string]domain.CleanCheckEntry),
	}
	return f.run()
}

type cleanFlow struct {
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter
	// checks query the PR state over the network, so each is made once.
	checks   map[string]domain.CleanCheckEntry
	holdings owed.Holdings
	moves    flow.SetMemo[[]domain.ReparentResult]
}

func (f *cleanFlow) run() (Outcome, error) {
	named := len(f.request.Branches) > 0
	requested, err := f.acceptRequested()
	if err != nil {
		return Outcome{}, err
	}
	f.request.Branches = requested.Present
	if named && len(requested.Present) == 0 {
		return f.conclude(concludeParams{Outcome: Outcome{Results: requested.Absent}})
	}

	// Checked before the first question, so the recap of a named worktree needs
	// no I/O. The prompt-free path checks in resolveDelete instead.
	if named && f.prompter.Interactive() {
		if err := f.checkAll(requested.Present); err != nil {
			return Outcome{}, err
		}
	}

	answers, err := f.prompter.Ask(f.session())
	if errors.Is(err, domain.ErrUserAborted) {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
	}

	selected := answers.Values(KeyWorktree)
	force := f.request.Force || answers.Value(KeyDelete) == deleteForce
	var skipped []domain.PruneSkip
	if answers.Value(KeyDelete) == deleteSafe {
		selected, skipped = f.splitUnsafe(selected)
	}
	// Read before the first removal, while every worktree still names its parent.
	nodes, nodesErr := worktree.Nodes(worktree.NodesParams{ProjectDir: f.ctx.ProjectDir, StateDir: f.ctx.StateDir})

	batch := len(selected) > 1
	removals := teardown.Batch(teardown.BatchParams{
		Context:      f.ctx,
		Presenter:    f.presenter,
		Targets:      f.targets(selected),
		Force:        force,
		ForceRemoval: force,
		BaseBranch:   f.request.BaseBranch,
		StartDown:    answers.Value(KeyData) == owed.DataStart,
		KeepData:     f.request.KeepData,
		Recover:      f.recoverRemoveFailure,
		OnStart: func(progress flow.Progress) {
			if batch {
				f.presenter.WorktreeStarted(progress)
			}
		},
		OnDone: func(removal teardown.Removal) {
			if !batch {
				return
			}
			if removal.Err != nil {
				f.presenter.WorktreeFailed(failureOf(removal))
				return
			}
			f.presenter.WorktreeCleaned(resultOf(removal))
		},
	})

	outcome := Outcome{Results: requested.Absent, Skipped: skipped}
	var first error
	for _, removal := range removals {
		if removal.Err != nil {
			outcome.Failed = append(outcome.Failed, failureOf(removal))
			if first == nil {
				first = removal.Err
			}
			continue
		}
		outcome.Results = append(outcome.Results, resultOf(removal))
		outcome.Namespaces = append(outcome.Namespaces, removal.Namespaces...)
	}

	// What actually went, not what was asked: a parent that failed to go is still
	// there for its children, and a child that failed to go is orphaned in turn.
	var moved []domain.ReparentResult
	if nodesErr == nil {
		moved = rules.ReparentsAfterRemoval(rules.ReparentsAfterRemovalParams{
			Nodes:      nodes,
			Removed:    removedBranches(removals),
			BaseBranch: f.request.BaseBranch,
		})
	}
	if answers.Value(KeyReparent) == orphans.Reparent && len(moved) > 0 {
		applied, err := worktree.ApplyReparents(worktree.ApplyReparentsParams{Reparents: moved, StateDir: f.ctx.StateDir})
		if err != nil {
			return Outcome{}, err
		}
		outcome.Reparented = applied
	} else {
		outcome.Orphaned = moved
	}
	return f.conclude(concludeParams{Outcome: outcome, First: first})
}

func resultOf(removal teardown.Removal) domain.CleanResult {
	return domain.CleanResult{Branch: removal.Target.Branch, Path: removal.Target.Path, AlreadyAbsent: removal.Absent}
}

func failureOf(removal teardown.Removal) domain.BatchFailure {
	return rules.BatchFailureOf(rules.BatchFailureOfParams{Branch: removal.Target.Branch, Path: removal.Target.Path, Err: removal.Err})
}

type concludeParams struct {
	Outcome Outcome
	First   error
}

func (f *cleanFlow) conclude(params concludeParams) (Outcome, error) {
	outcome := params.Outcome
	if err := f.presenter.Cleaned(outcome); err != nil {
		return outcome, err
	}
	return outcome, flow.BatchError(flow.BatchErrorParams{
		First: params.First,
		Batch: len(outcome.Results)+len(outcome.Failed)+len(outcome.Skipped) > 1,
	})
}

type requestedWorktrees struct {
	Present []string
	Absent  []domain.CleanResult
}

// acceptRequested reads git's own list rather than a check, which would ask the
// network: an absent worktree is concluded as such, the parent is left out with
// a warning, and a blank or repeated argument is a malformed invocation (exit 2).
func (f *cleanFlow) acceptRequested() (requestedWorktrees, error) {
	var requested requestedWorktrees
	if len(f.request.Branches) == 0 {
		return requested, nil
	}
	names, err := rules.DistinctNames(rules.DistinctNamesParams{Names: f.request.Branches, Blank: domain.CleanBranchBlank})
	if err != nil {
		return requested, err
	}
	worktrees, err := worktree.ListAll(worktree.ListAllParams{ProjectDir: f.ctx.ProjectDir})
	if err != nil {
		return requested, fmt.Errorf("list worktrees: %w", err)
	}
	byBranch := make(map[string]domain.GitWorktree, len(worktrees))
	for _, wt := range worktrees {
		byBranch[wt.Branch] = wt
	}

	for _, name := range names {
		wt, exists := byBranch[name]
		switch {
		case !exists:
			requested.Absent = append(requested.Absent, domain.CleanResult{Branch: name, AlreadyAbsent: true})
		case wt.IsMain:
			f.presenter.Notice(flow.Notice{Kind: flow.NoticeWarning, Text: domain.CleanCannotCleanParent})
		default:
			requested.Present = append(requested.Present, name)
		}
	}
	return requested, nil
}

func (f *cleanFlow) targets(selected []string) []teardown.Target {
	targets := make([]teardown.Target, 0, len(selected))
	for _, branch := range selected {
		target := teardown.Target{Branch: branch}
		if wt, err := worktree.FindByBranch(worktree.FindByBranchParams{ProjectDir: f.ctx.ProjectDir, Branch: branch}); err == nil {
			target.Path = wt.Path
		}
		targets = append(targets, target)
	}
	return targets
}

func (f *cleanFlow) splitUnsafe(selected []string) ([]string, []domain.PruneSkip) {
	var safe []string
	var skipped []domain.PruneSkip
	for _, branch := range selected {
		if reason := rules.CleanSkipReason(f.checks[branch].Check); reason != "" {
			skipped = append(skipped, domain.PruneSkip{Branch: branch, Reason: reason})
			continue
		}
		safe = append(safe, branch)
	}
	return safe, skipped
}

// Force plays no part in the moves a removal makes necessary.
func (f *cleanFlow) reparents(selected []string) []domain.ReparentResult {
	return f.moves.Get(selected, func() []domain.ReparentResult {
		nodes, err := worktree.Nodes(worktree.NodesParams{ProjectDir: f.ctx.ProjectDir, StateDir: f.ctx.StateDir})
		if err != nil {
			return nil
		}
		return rules.ReparentsAfterRemoval(rules.ReparentsAfterRemovalParams{
			Nodes:      nodes,
			Removed:    selected,
			BaseBranch: f.request.BaseBranch,
		})
	})
}

func removedBranches(removals []teardown.Removal) []string {
	var removed []string
	for _, removal := range removals {
		if removal.Err == nil {
			removed = append(removed, removal.Target.Branch)
		}
	}
	return removed
}

// recoverRemoveFailure offers the privileged removal when `git worktree remove`
// failed on files the current user cannot delete (typically root-owned files
// left by a container). Declined or out of reach, what git did is settled as it
// stands.
func (f *cleanFlow) recoverRemoveFailure(salvage teardown.SalvageParams) error {
	if !f.request.AllowPrivileged || !f.prompter.Interactive() || salvage.Path == "" {
		return teardown.Salvage(salvage)
	}

	f.presenter.Status(flow.Notice{
		Kind: flow.NoticeWarning,
		Text: fmt.Sprintf(domain.CleanRemovalFailedFmt, salvage.Cause),
	})

	confirmed, err := f.prompter.Confirm(flow.ConfirmParams{
		Title:      fmt.Sprintf(domain.CleanSudoConfirmFmt, salvage.Path),
		DefaultYes: false,
	})
	if err != nil || !confirmed {
		return teardown.Salvage(salvage)
	}

	return worktree.ForceClean(domain.ForceCleanParams{
		ProjectDir: salvage.Clean.ProjectDir,
		StateDir:   salvage.Clean.StateDir,
		Path:       salvage.Path,
		Branch:     salvage.Clean.Branch,
		Force:      salvage.Clean.Force,
	})
}

// checkAll shows its own progress; checksOf, read while a step loads, leaves it
// to the host, which already shows one.
func (f *cleanFlow) checkAll(branches []string) error {
	missing := f.unchecked(branches)
	if len(missing) == 0 {
		return nil
	}
	return f.presenter.Stage(flow.StageParams{
		Message: domain.CleanCheckLoading,
		Work: func() error {
			f.fetchChecks(missing)
			return nil
		},
	})
}

// checksOf leaves out a worktree gone or turned parent since git listed it,
// which the removal reports on its own. Any other failure comes back beside the
// checks that did read: a recap that folded a worktree out would confirm a
// removal it never showed, and a refusal must still see the others.
func (f *cleanFlow) checksOf(branches []string) ([]domain.CleanCheckResult, error) {
	f.fetchChecks(f.unchecked(branches))
	checks := make([]domain.CleanCheckResult, 0, len(branches))
	var unreadable error
	for _, branch := range branches {
		entry := f.checks[branch]
		switch {
		case entry.Err == nil:
			checks = append(checks, entry.Check)
		case errors.Is(entry.Err, domain.ErrWorktreeNotFound), errors.Is(entry.Err, domain.ErrCannotCleanParent):
		case unreadable == nil:
			unreadable = fmt.Errorf(domain.CleanCheckFailedFmt, branch, entry.Err)
		}
	}
	return checks, unreadable
}

func (f *cleanFlow) unchecked(branches []string) []string {
	var missing []string
	for _, branch := range branches {
		if _, cached := f.checks[branch]; !cached {
			missing = append(missing, branch)
		}
	}
	return missing
}

func (f *cleanFlow) fetchChecks(branches []string) {
	if len(branches) == 0 {
		return
	}
	for branch, entry := range worktree.CheckAll(worktree.CheckAllParams{ProjectDir: f.ctx.ProjectDir, Branches: branches}) {
		f.checks[branch] = entry
	}
}
