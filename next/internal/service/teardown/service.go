package teardown

import (
	"context"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The service

// Service checks and removes worktrees for the commands that remove them. Every field must be set before Remove.
type Service struct {
	Git       Git
	Meta      Meta
	EnvFiles  EnvFiles
	Daemon    Daemon
	Owed      Owed
	StateDir  string // wtm's state, where each worktree's hook log and job logs go
	CommonDir string // the repository's common directory, which the events of a job started to drop data name

	Gone      func(ctx context.Context, w worktree.Worktree) (bool, error) // whether w's directory is gone (repository.Gone)
	RunHooks  func(ctx context.Context, chain env.HookChain) error         // runs the on_clean hooks and a namespace's remove command, failing with an *env.HookFailure
	PurgeLogs func(dir string) error                                       // removes a worktree's job logs
	Shield    func(ctx context.Context) (context.Context, func())          // a context the first cancel does not reach, and its release
}

// ----------------------------------------------------------------------------
// The checks

// ReadParams are the worktrees Read checks, and the branches going with them, which hold nothing for each other; every other branch stays.
type ReadParams struct {
	Worktrees []worktree.Worktree
	Going     []string
}

// Read reads, for each worktree of params, whether its directory is gone (a locked one git never calls prunable among them), its uncommitted changes while it is there, and the commits of its branch and of the commit it stands on no branch that stays holds, a commit on no branch or of a rebase under way among them, its directory gone or not. What wtm's daemon and the forge say is Settle's.
func (s Service) Read(ctx context.Context, params ReadParams) (map[string]Check, error) {
	going := params.Going
	checks := map[string]Check{}
	for _, w := range params.Worktrees {
		var check Check
		var err error
		if w.Path != "" {
			if check.Missing, err = s.Gone(ctx, w); err != nil {
				return nil, err
			}
		}
		if w.Path != "" && !check.Missing {
			if check.Dirty, err = s.Git.Dirty(ctx, w.Path); err != nil {
				return nil, err
			}
		}
		if losing := (worktree.Losing{Branch: w.Branch(), Head: w.Head, Going: going}); losing.Branch != "" || losing.Head != "" {
			if check.Unpushed, err = s.Git.Unpushed(ctx, losing); err != nil {
				return nil, err
			}
		}
		checks[w.Name()] = check
	}

	return checks, nil
}

// ----------------------------------------------------------------------------
// The removal

// The codes of what becomes of a removal, after the command's name: jobs that would not stop, or left running when forced, followed by their stop (StopLeft); an on_clean hook that failed, the hooks skipped or run without the worktree's environment, a worktree git could not remove or whose files stay on disk, a branch or a record that could not be deleted.
const (
	StopFailed         = ".stop_failed"
	JobsLeft           = ".jobs_left"
	StopLeft           = ".down"
	HookFailed         = ".hook_failed"
	HooksSkipped       = ".hooks_skipped"
	HookEnvUnavailable = ".hook_env_unavailable"
	RemoveFailed       = ".remove_failed"
	LeftOnDisk         = ".left_on_disk"
	BranchKept         = ".branch_kept"
	MetaKept           = ".meta_kept"
)

// BatchParams are the command removing, the facts it planned on, the removals in order, whether a worktree whose jobs would not stop is removed all the same, the operation's correlation id, which wtm's daemon's events carry, and where the events go.
type BatchParams struct {
	Command       string
	Facts         Facts
	Removals      []Removal
	Force         bool
	CorrelationID string
	Report        domain.Report
}

// Batch removes each of the removals in turn, a failure stopping none of the others: first the shared services down their data asks started are started, then each removal (Remove), then, once every drop is made, what the worktrees removed held in the shared services is let go of, and the services started stopped again. It returns what became of each removal, in order, and the warnings of the batch.
func (s Service) Batch(ctx context.Context, params BatchParams) ([]Removed, []domain.Warning) {
	lit, unlit := s.light(ctx, params)
	removed := make([]Removed, 0, len(params.Removals))
	for _, removal := range params.Removals {
		removed = append(removed, s.Remove(ctx, RemoveParams{Command: params.Command, Facts: params.Facts, Removal: removal, Force: params.Force, CorrelationID: params.CorrelationID, Unlit: unlit, Report: params.Report}))
	}

	// What went is let go of, interrupted or not: nothing would release it later.
	shielded, release := s.Shield(ctx)
	defer release()
	return removed, s.release(shielded, params, removed, lit)
}

// RemoveParams are the command removing, the facts it planned on, the removal, whether it goes when its jobs would not stop, the operation's correlation id, why each shared service the batch was to start for its data could not start, and where its events go.
type RemoveParams struct {
	Command       string
	Facts         Facts
	Removal       Removal
	Force         bool
	CorrelationID string
	Unlit         map[string]error
	Report        domain.Report
}

// Removed is what became of one removal: its status and why, the error it failed on, its warnings, whether its worktree went, its branch deleted or not, and what became of its data.
type Removed struct {
	Status     domain.Status
	Reason     domain.Reason
	Err        *domain.Error
	Warnings   []domain.Warning
	Gone       bool
	Namespaces []Dropped
}

// Remove removes one worktree between its unit events: its own jobs stopped, a job left running refusing its removal unless forced; its on_clean hooks, which refuse its removal when one fails; then, shielded from a cancel, the worktree, its branch and its meta record, the event that names it gone, its data dropped or owed and its job logs purged. A worktree reached after a cancel is skipped. Its claims on the shared services are let go of by Batch, once every drop is made.
func (s Service) Remove(ctx context.Context, params RemoveParams) (removed Removed) {
	removal, report := params.Removal, params.Report
	if ctx.Err() != nil {
		return Removed{Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted}
	}

	report(domain.UnitStarted{Subject: removal.Worktree})
	defer func() { report(domain.UnitFinished{Subject: removal.Worktree, Status: removed.Status}) }()

	stopWarnings, stopErr := s.stopJobs(ctx, params)
	if stopErr != nil {
		removed = Removed{Status: domain.StatusFailed, Err: stopErr}
		if ctx.Err() != nil {
			removed.Status, removed.Reason = domain.StatusCancelled, domain.ReasonInterrupted
		}
		return removed
	}

	warnings, err := s.hooks(ctx, params)
	warnings = append(stopWarnings, warnings...)
	if err != nil {
		removed = Removed{Status: domain.StatusFailed, Err: failure(params.Command+HookFailed, "on_clean hook failed", err), Warnings: warnings}
		if ctx.Err() != nil {
			removed.Status, removed.Reason = domain.StatusCancelled, domain.ReasonInterrupted
		}
		return removed
	}

	// Shielded: a cancel half-way would strand a branch and a record with no worktree, and an event nobody publishes.
	shielded, release := s.Shield(ctx)
	defer release()
	report(domain.PhaseStarted{Subject: removal.Worktree, Phase: domain.PhaseRemove})
	removed = s.removeWorktree(shielded, params)
	report(domain.PhaseFinished{Subject: removal.Worktree, Phase: domain.PhaseRemove, Failed: removed.Status != domain.StatusDone})

	removed.Warnings = append(warnings, removed.Warnings...)
	if !removed.Gone {
		return removed
	}
	dropped, dataWarnings := s.dropData(shielded, params)
	removed.Namespaces, removed.Warnings = dropped, append(removed.Warnings, dataWarnings...)
	// Best effort: a log left over is not worth failing a removal that happened.
	_ = s.PurgeLogs(run.LogDir(s.StateDir, removal.Branch))
	return removed
}

// stopJobs stops the jobs the worktree runs for itself, between the events of its phase, its claims on shared services left for its data's drop; then asks wtm's daemon again: a job still up, or a daemon that cannot tell, fails the removal unless forced, warned then.
func (s Service) stopJobs(ctx context.Context, params RemoveParams) ([]domain.Warning, *domain.Error) {
	removal, report := params.Removal, params.Report
	if !slices.Contains(removal.Phases, domain.PhaseStopJobs) {
		return nil, nil
	}

	report(domain.PhaseStarted{Subject: removal.Worktree, Phase: domain.PhaseStopJobs})
	names := removal.Jobs
	if jobs, err := s.jobs(ctx); err == nil {
		names = run.OwnUpIn(jobs, removal.Path)
	}
	for _, name := range names {
		// A stop that fails shows in the listing after.
		_, _ = s.Daemon.Stop(ctx, run.Stop{Name: name, WorkDir: removal.Path, CorrelationID: params.CorrelationID}, nil)
	}
	jobs, err := s.jobs(ctx)
	left := run.OwnUpIn(jobs, removal.Path)
	report(domain.PhaseFinished{Subject: removal.Worktree, Phase: domain.PhaseStopJobs, Failed: err != nil || len(left) > 0})

	if err == nil && len(left) == 0 {
		return nil, nil
	}
	about := domain.Params{"worktree": removal.Worktree}
	still := strings.Join(left, ", ") + " would not stop"
	if err != nil {
		still = "whether its jobs stopped is unknown: " + run.UnknownMessage(err)
	} else {
		about["jobs"] = strings.Join(left, ", ")
	}
	if params.Force {
		down := &domain.FollowUp{Command: domain.CommandRunDown, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{removal.Path}}}, Code: params.Command + StopLeft, Message: "stop what it left running"}
		return []domain.Warning{{Code: params.Command + JobsLeft, Message: removal.Worktree + " is removed all the same: " + still, Params: about, FollowUp: down}}, nil
	}
	return nil, &domain.Error{Kind: domain.KindInternal, Code: params.Command + StopFailed, Message: removal.Worktree + " is kept: " + still, Params: about}
}

// hooks runs the on_clean hooks in the worktree, their variables expanded and the worktree's run environment set, between their phase's events; a worktree whose directory is gone runs none, with a warning.
func (s Service) hooks(ctx context.Context, params RemoveParams) ([]domain.Warning, error) {
	facts, removal, report := params.Facts, params.Removal, params.Report
	hooks := facts.onClean(removal)
	switch {
	case len(hooks) == 0 && removal.Missing && len(facts.Config.OnClean) > 0:
		return []domain.Warning{{Code: params.Command + HooksSkipped, Message: "its directory is gone: the on_clean hooks did not run", Params: domain.Params{"path": removal.Path}}}, nil
	case len(hooks) == 0:
		return nil, nil
	}

	log := env.HookLogPath(env.HookLogPathParams{StateDir: s.StateDir, Branch: removal.Branch, Hooks: env.OnCleanHooks})
	hookEnv, warnings := s.hookEnv(params)
	report(domain.PhaseStarted{Subject: removal.Worktree, Phase: domain.PhaseOnClean, Log: log})
	err := s.RunHooks(ctx, env.HookChain{
		Hooks:   hooks,
		WorkDir: removal.Path,
		Env:     hookEnv,
		Unset:   env.NotInherited(facts.Run),
		Output:  func(line string) { report(domain.Output{Subject: removal.Worktree, Line: line}) },
		Log:     log,
	})
	report(domain.PhaseFinished{Subject: removal.Worktree, Phase: domain.PhaseOnClean, Failed: err != nil})
	return warnings, err
}

// hookEnv is the run environment the hooks get, as the worktree's own jobs get it (its Compose project, its ports): nil when they read none, or with a warning when a compose env file cannot be read.
func (s Service) hookEnv(params RemoveParams) (map[string]string, []domain.Warning) {
	facts, removal := params.Facts, params.Removal
	record := facts.Records[removal.Branch]
	isolation := record.Isolation.Effective()
	if !env.HooksReadRun(facts.Run, isolation) {
		return nil, nil
	}

	job := env.JobParams{Branch: removal.Branch, ProjectDir: facts.MainPath, Ordinal: record.Ordinal, Isolation: isolation, Run: facts.Run}
	hookEnv, err := env.ReadHookEnv(job, removal.Path, s.EnvFiles.Read)
	if err != nil {
		return nil, []domain.Warning{{Code: params.Command + HookEnvUnavailable, Message: err.Error() + " — on_clean hooks run without the worktree's environment", Params: domain.Params{"cause": err.Error()}}}
	}
	return hookEnv, nil
}

// removeWorktree removes the worktree, then its branch and its meta record, and reports it removed; a branch no worktree holds is deleted alone, with its record. A worktree git dropped while files it could not delete stay on disk (a container's, owned by root) is removed all the same, those files named in a warning.
func (s Service) removeWorktree(ctx context.Context, params RemoveParams) Removed {
	removal := params.Removal
	removed := Removed{Status: domain.StatusDone, Gone: true}
	if removal.Path == "" {
		return s.deleteBranch(ctx, params, removed)
	}

	if err := s.Git.RemoveWorktree(ctx, removal.Path); err != nil {
		if s.listed(ctx, removal.Path) {
			return Removed{Status: domain.StatusFailed, Err: failure(params.Command+RemoveFailed, "couldn't remove the worktree", err)}
		}
		removed.Warnings = append(removed.Warnings, domain.Warning{Code: params.Command + LeftOnDisk, Message: removal.Path + " is still on disk (" + err.Error() + "): delete what is left with sudo rm -rf " + removal.Path, Params: domain.Params{"path": removal.Path}})
	}

	params.Report(domain.WorktreeRemoved{Worktree: worktree.IdentityOf(worktree.IdentityParams{Branch: removal.Branch, Path: removal.Path, Record: params.Facts.Records[removal.Branch]})})
	if removal.Branch == "" {
		return removed
	}

	return s.deleteBranch(ctx, params, removed)
}

// deleteBranch deletes the branch of the removal and its meta record, once its worktree is gone or when it had none; a branch with no commit yet has no ref to delete.
func (s Service) deleteBranch(ctx context.Context, params RemoveParams, removed Removed) Removed {
	removal := params.Removal
	var err error
	if params.Facts.Local(removal.Branch) {
		err = s.Git.DeleteBranch(ctx, removal.Branch)
	}
	if err != nil && removal.Path == "" {
		return Removed{Status: domain.StatusFailed, Err: failure(params.Command+BranchKept, "couldn't delete the branch "+removal.Branch, err)}
	} else if err != nil {
		removed.Status, removed.Reason, removed.Err = domain.StatusFailed, domain.ReasonIncomplete, failure(params.Command+BranchKept, "the worktree is removed but its branch "+removal.Branch+" is kept", err)
		return removed
	}
	if err := s.Meta.Delete(removal.Branch); err != nil {
		removed.Warnings = append(removed.Warnings, domain.Warning{Code: params.Command + MetaKept, Message: "couldn't delete the meta record of " + removal.Branch + ": " + err.Error(), Params: domain.Params{"branch": removal.Branch}})
	}

	return removed
}

// listed reports whether git still lists a worktree at path; a list git cannot give counts as listed, so nothing is taken as removed that may not be.
func (s Service) listed(ctx context.Context, path string) bool {
	worktrees, err := s.Git.Worktrees(ctx)
	return err != nil || slices.ContainsFunc(worktrees, func(w worktree.Worktree) bool { return w.Path == path })
}

// failure is the error a removal failed on, coded as the command's.
func failure(code, message string, err error) *domain.Error {
	return domain.AsError(domain.AsErrorParams{Err: err, Code: code, Message: message})
}
