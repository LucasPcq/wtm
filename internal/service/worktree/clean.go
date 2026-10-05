package worktree

import (
	"context"
	"fmt"
	"os"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/hooks"
)

type CheckAllParams struct {
	ProjectDir string
	Branches   []string
}

// CheckAll performs the pre-deletion checks of several worktrees, asking GitHub
// once for all of them rather than once each — unless that one list was cut
// short, where a branch missing from it is asked on its own: an open pull
// request is a refusal, and a refusal is never waived for want of a page.
func CheckAll(ctx context.Context, params CheckAllParams) map[string]domain.CleanCheckEntry {
	open, complete := ghservice.OpenPRsByBranch(ctx, params.ProjectDir)
	entries := make(map[string]domain.CleanCheckEntry, len(params.Branches))
	for _, branch := range params.Branches {
		check, err := checkLocal(ctx, checkLocalParams{ProjectDir: params.ProjectDir, Branch: branch})
		if err == nil {
			check.HasOpenPR, check.PRUrl = cleanOpenPR(ctx, cleanOpenPRParams{ProjectDir: params.ProjectDir, Branch: branch, Open: open, Complete: complete})
		}
		entries[branch] = domain.CleanCheckEntry{Check: check, Err: err}
	}
	return entries
}

type cleanOpenPRParams struct {
	ProjectDir string
	Branch     string
	Open       map[string]string
	Complete   bool
}

func cleanOpenPR(ctx context.Context, params cleanOpenPRParams) (bool, string) {
	if url, found := params.Open[params.Branch]; found {
		return true, url
	}
	if params.Complete {
		return false, ""
	}
	found, _, url := ghservice.HasOpenPR(ctx, ghservice.HasOpenPRParams{ProjectDir: params.ProjectDir, Branch: params.Branch})
	return found, url
}

type checkLocalParams struct {
	ProjectDir string
	Branch     string
}

func checkLocal(ctx context.Context, params checkLocalParams) (domain.CleanCheckResult, error) {
	wt, err := infra.FindWorktreeByBranch(ctx, infra.FindWorktreeByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})
	if err != nil {
		return domain.CleanCheckResult{}, err
	}
	if wt.IsMain {
		return domain.CleanCheckResult{}, domain.ErrCannotCleanParent
	}
	unpushed, _ := infra.UnpushedCommits(ctx, infra.UnpushedCommitsParams{ProjectDir: params.ProjectDir, Branch: params.Branch})
	dirty, _ := infra.IsDirty(ctx, infra.IsDirtyParams{WorktreePath: wt.Path})
	return domain.CleanCheckResult{
		WorktreePath:    wt.Path,
		Branch:          params.Branch,
		UnpushedCommits: unpushed,
		IsDirty:         dirty,
		IsLocked:        wt.Locked,
	}, nil
}

// Clean removes the worktree and deletes the local branch.
func Clean(ctx context.Context, params domain.CleanParams) error {
	wt, err := infra.FindWorktreeByBranch(ctx, infra.FindWorktreeByBranchParams{
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
		if err := RunCleanHooks(ctx, domain.CleanHooksParams{
			ProjectDir:   params.ProjectDir,
			StateDir:     params.StateDir,
			WorktreePath: wt.Path,
			Branch:       params.Branch,
			Hooks:        params.Config.Project.Hooks.OnClean,
		}); err != nil {
			return err
		}
	}

	if err := infra.RemoveWorktree(ctx, infra.RemoveWorktreeParams{
		ProjectDir: params.ProjectDir,
		Path:       wt.Path,
		Force:      params.Force,
		Locked:     wt.Locked,
	}); err != nil {
		return fmt.Errorf("%w: %w", domain.ErrWorktreeRemoveFailed, err)
	}

	if err := infra.DeleteLocalBranch(ctx, infra.DeleteLocalBranchParams{
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
func RunCleanHooks(ctx context.Context, params domain.CleanHooksParams) error {
	if len(params.Hooks) == 0 {
		return nil
	}

	mainPath, err := infra.FindMainWorktreePath(ctx, infra.FindMainWorktreeParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return fmt.Errorf("find main checkout: %w", err)
	}

	if err := hooks.RunHooks(ctx, hooks.RunHooksParams{
		Hooks:   params.Hooks,
		WorkDir: params.WorktreePath,
		Vars: rules.TemplateVars{
			Worktree: params.WorktreePath,
			Branch:   params.Branch,
			Root:     mainPath,
		},
		Env: hookEnv(ctx, hookEnvParams{
			Ref:          WorktreeRef{ProjectDir: params.ProjectDir, StateDir: params.StateDir, Branch: params.Branch},
			WorktreePath: params.WorktreePath,
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
func ForceClean(ctx context.Context, params domain.ForceCleanParams) error {
	homeDir, _ := os.UserHomeDir()
	if err := rules.ValidateSudoDeletePath(rules.SudoDeletePathParams{
		Path:       params.Path,
		HomeDir:    homeDir,
		ProjectDir: params.ProjectDir,
	}); err != nil {
		return err
	}

	if err := infra.SudoDeleteDir(ctx, infra.SudoDeleteDirParams{Path: params.Path}); err != nil {
		return err
	}

	if err := infra.PruneWorktrees(ctx, params.ProjectDir); err != nil {
		return fmt.Errorf("prune worktrees: %w", err)
	}

	if err := infra.DeleteLocalBranch(ctx, infra.DeleteLocalBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
		Force:      params.Force,
	}); err != nil {
		return fmt.Errorf("delete branch: %w", err)
	}

	purgeState(WorktreeRef{ProjectDir: params.ProjectDir, StateDir: params.StateDir, Branch: params.Branch})

	return nil
}
