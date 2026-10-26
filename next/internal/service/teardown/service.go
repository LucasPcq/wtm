package teardown

import (
	"context"
	"path/filepath"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The service

// Service checks and removes worktrees for the commands that remove them. Every field must be set before Remove.
type Service struct {
	Git      Git
	Meta     Meta
	EnvFiles EnvFiles
	StateDir string // wtm's state, where each worktree's hook log goes

	RunHooks func(ctx context.Context, chain env.HookChain) error // runs the on_clean hooks, failing with an *env.HookFailure
	Shield   func(ctx context.Context) (context.Context, func())  // a context the first cancel does not reach, and its release
}

// ----------------------------------------------------------------------------
// The checks

// Read reads, for each of worktrees, whether its directory is gone (a locked one git never calls prunable among them), its uncommitted changes while it is there, and the commits of its branch and of the commit it stands on no branch that stays holds, a commit on no branch or of a rebase under way among them, its directory gone or not. Worktrees removed together hold nothing for each other. What the run daemon and the forge say is Settle's.
func (s Service) Read(ctx context.Context, worktrees []worktree.Worktree) (map[string]Check, error) {
	var going []string
	for _, w := range worktrees {
		going = append(going, w.Branch())
	}

	checks := map[string]Check{}
	for _, w := range worktrees {
		var check Check
		var err error
		if w.Path != "" {
			if check.Missing, err = s.gone(ctx, w); err != nil {
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

// gone reports whether the directory of w is gone: git calls it prunable, or nothing is there.
func (s Service) gone(ctx context.Context, w worktree.Worktree) (bool, error) {
	// git never lists a locked worktree prunable, even with its directory gone: the disk is asked.
	if w.Prunable {
		return true, nil
	}
	location, err := s.Git.Locate(ctx, w.Path)
	if err != nil {
		return false, err
	}

	return !location.Exists, nil
}

// ----------------------------------------------------------------------------
// The removal

// The codes of what becomes of a removal, after the command's name: an on_clean hook that failed, the hooks skipped or run without the worktree's environment, a worktree git could not remove or whose files stay on disk, a branch or a record that could not be deleted.
const (
	HookFailed         = ".hook_failed"
	HooksSkipped       = ".hooks_skipped"
	HookEnvUnavailable = ".hook_env_unavailable"
	RemoveFailed       = ".remove_failed"
	LeftOnDisk         = ".left_on_disk"
	BranchKept         = ".branch_kept"
	MetaKept           = ".meta_kept"
)

// RemoveParams are the command removing, the facts it planned on, the removal and where its events go.
type RemoveParams struct {
	Command string
	Facts   Facts
	Removal Removal
	Report  domain.Report
}

// Removed is what became of one removal: its status and why, the error it failed on, its warnings, and whether its worktree went, its branch deleted or not.
type Removed struct {
	Status   domain.Status
	Reason   domain.Reason
	Err      *domain.Error
	Warnings []domain.Warning
	Gone     bool
}

// Remove removes one worktree between its unit events: its on_clean hooks, which refuse its removal when one fails, then the worktree, its branch and its meta record, shielded from a cancel, then the event that names it gone. A worktree reached after a cancel is skipped.
func (s Service) Remove(ctx context.Context, params RemoveParams) (removed Removed) {
	removal, report := params.Removal, params.Report
	if ctx.Err() != nil {
		return Removed{Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted}
	}

	report(domain.UnitStarted{Subject: removal.Worktree})
	defer func() { report(domain.UnitFinished{Subject: removal.Worktree, Status: removed.Status}) }()

	warnings, err := s.hooks(ctx, params)
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
	return removed
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
		Unset:   env.JobKeys(),
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

	var compose [][]dotenv.Line
	for _, path := range env.ComposeEnvPaths(facts.Run) {
		lines, err := s.EnvFiles.Read(filepath.Join(removal.Path, path))
		if err != nil {
			return nil, []domain.Warning{{Code: params.Command + HookEnvUnavailable, Message: err.Error() + " — on_clean hooks run without the worktree's environment", Params: domain.Params{"cause": err.Error()}}}
		}
		compose = append(compose, lines)
	}

	return env.HookEnv(env.JobParams{Branch: removal.Branch, ProjectDir: facts.MainPath, Ordinal: record.Ordinal, Isolation: isolation, Run: facts.Run}, compose), nil
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
