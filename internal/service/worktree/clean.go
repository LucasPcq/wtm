package worktree

import (
	"fmt"
	"os"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/hooks"
)

// Check performs pre-deletion checks without deleting anything.
func Check(params domain.CleanParams) (domain.CleanCheckResult, error) {
	wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})
	if err != nil {
		return domain.CleanCheckResult{}, err
	}

	if wt.IsMain {
		return domain.CleanCheckResult{}, domain.ErrCannotCleanParent
	}

	unpushed, _ := infra.UnpushedCommits(infra.UnpushedCommitsParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})

	haspr, _, prurl := ghservice.HasOpenPR(ghservice.HasOpenPRParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})

	dirty, _ := infra.IsDirty(infra.IsDirtyParams{WorktreePath: wt.Path})

	return domain.CleanCheckResult{
		WorktreePath:    wt.Path,
		Branch:          params.Branch,
		UnpushedCommits: unpushed,
		HasOpenPR:       haspr,
		PRUrl:           prurl,
		IsDirty:         dirty,
		IsParent:        wt.IsMain,
	}, nil
}

// Clean removes the worktree and deletes the local branch.
func Clean(params domain.CleanParams) error {
	wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})
	if err != nil {
		return err
	}

	if wt.IsMain {
		return domain.ErrCannotCleanParent
	}

	// on_clean hooks run inline unless the caller opts to run them as a separate
	// phase (clean's phased output) via SkipHooks.
	if !params.SkipHooks {
		if err := RunCleanHooks(domain.CleanHooksParams{
			ProjectDir:   params.ProjectDir,
			StateDir:     params.StateDir,
			WorktreePath: wt.Path,
			Branch:       params.Branch,
			Hooks:        params.Config.Project.Hooks.OnClean,
		}); err != nil {
			return err
		}
	}

	if err := infra.RemoveWorktree(infra.RemoveWorktreeParams{
		ProjectDir: params.ProjectDir,
		Path:       wt.Path,
		Force:      params.Force,
	}); err != nil {
		return fmt.Errorf("%w: %w", domain.ErrWorktreeRemoveFailed, err)
	}

	if err := infra.DeleteLocalBranch(infra.DeleteLocalBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
		Force:      params.Force,
	}); err != nil {
		return fmt.Errorf("delete branch: %w", err)
	}

	purgeState(WorktreeRef{ProjectDir: params.ProjectDir, StateDir: params.StateDir, Branch: params.Branch})

	return nil
}

// RunCleanHooks runs the configured on_clean hooks in the worktree directory
// before it is removed (e.g. `docker compose down`). It runs after wtm has
// stopped its own services and before the directory is deleted, so hooks can
// still reference files being removed. A failing hook aborts the removal unless
// the entry sets continue_on_error. Exposed so `clean` can run them as a distinct,
// titled phase before the removal spinner.
func RunCleanHooks(params domain.CleanHooksParams) error {
	if len(params.Hooks) == 0 {
		return nil
	}

	mainPath, err := infra.FindMainWorktreePath(infra.FindMainWorktreeParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return fmt.Errorf("find main worktree: %w", err)
	}

	if err := hooks.RunHooks(hooks.RunHooksParams{
		Hooks:   params.Hooks,
		WorkDir: params.WorktreePath,
		Vars: rules.TemplateVars{
			Worktree: params.WorktreePath,
			Branch:   params.Branch,
			Root:     mainPath,
		},
		Env: hookEnv(WorktreeRef{
			ProjectDir: params.ProjectDir,
			StateDir:   params.StateDir,
			Branch:     params.Branch,
		}),
		Output: params.Output,
		OnHook: params.OnHook,
	}); err != nil {
		return fmt.Errorf("%s: %w", domain.HookOnClean, err)
	}

	return nil
}

// ForceClean recovers a worktree whose `git worktree remove` failed on
// undeletable files: it deletes the directory with `sudo rm -rf`, prunes the
// stale git metadata, then deletes the local branch. Intended to run only after
// the user has confirmed the privileged deletion.
func ForceClean(params domain.ForceCleanParams) error {
	homeDir, _ := os.UserHomeDir()
	if err := rules.ValidateSudoDeletePath(rules.SudoDeletePathParams{
		Path:       params.Path,
		HomeDir:    homeDir,
		ProjectDir: params.ProjectDir,
	}); err != nil {
		return err
	}

	if err := infra.SudoDeleteDir(infra.SudoDeleteDirParams{Path: params.Path}); err != nil {
		return err
	}

	if err := infra.PruneWorktrees(params.ProjectDir); err != nil {
		return fmt.Errorf("prune worktrees: %w", err)
	}

	if err := infra.DeleteLocalBranch(infra.DeleteLocalBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
		Force:      params.Force,
	}); err != nil {
		return fmt.Errorf("delete branch: %w", err)
	}

	purgeState(WorktreeRef{ProjectDir: params.ProjectDir, StateDir: params.StateDir, Branch: params.Branch})

	return nil
}
