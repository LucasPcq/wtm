package infra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ListWorktreesParams holds inputs for listing worktrees.
type ListWorktreesParams struct {
	ProjectDir string
}

// ListWorktrees returns all git worktrees with their path and branch.
func ListWorktrees(ctx context.Context, params ListWorktreesParams) ([]domain.GitWorktree, error) {
	cmd := Command(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = params.ProjectDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	var worktrees []domain.GitWorktree
	var current domain.GitWorktree
	isFirst := true

	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = domain.GitWorktree{
				Path:   strings.TrimPrefix(line, "worktree "),
				IsMain: isFirst,
			}
			isFirst = false
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "locked" || strings.HasPrefix(line, "locked "):
			current.Locked = true
		}
	}

	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	// A worktree with a rebase stopped mid-way (e.g. left in progress by
	// `wtm sync --keep-conflict`) has a detached HEAD, so `git worktree list` emits
	// no branch line. Recover the branch being rebased so the worktree still maps to
	// its branch for sync/tree/list (an empty branch would otherwise be read as an
	// invalid node and, chained through source-branch metadata, misreported as a cycle).
	for i := range worktrees {
		if worktrees[i].Branch != "" {
			continue
		}
		if branch, ok := rebaseInProgressBranch(ctx, worktrees[i].Path); ok {
			worktrees[i].Branch = branch
			worktrees[i].RebaseInProgress = true
		}
	}

	return worktrees, nil
}

// rebaseInProgressBranch returns the original branch of a rebase stopped mid-way
// in the worktree, read from the rebase state's head-name (interactive/merge
// rebase writes rebase-merge, the am-based rebase writes rebase-apply). It returns
// ok=false when no rebase is in progress.
func rebaseInProgressBranch(ctx context.Context, worktreePath string) (string, bool) {
	for _, stateDir := range []string{"rebase-merge", "rebase-apply"} {
		pathOut, err := Command(ctx, "git", "-C", worktreePath,
			"rev-parse", "--git-path", stateDir+"/head-name").Output()
		if err != nil {
			continue
		}
		headName := strings.TrimSpace(string(pathOut))
		if headName == "" {
			continue
		}
		// `git rev-parse --git-path` returns an absolute path for a linked worktree
		// but a path relative to the worktree (e.g. `.git/rebase-merge/head-name`)
		// for the main worktree. os.ReadFile resolves relative paths against the
		// process cwd, not the worktree, so anchor it explicitly.
		if !filepath.IsAbs(headName) {
			headName = filepath.Join(worktreePath, headName)
		}
		content, err := os.ReadFile(headName)
		if err != nil {
			continue
		}
		if branch := strings.TrimPrefix(strings.TrimSpace(string(content)), "refs/heads/"); branch != "" {
			return branch, true
		}
	}
	return "", false
}

// FindMainWorktreeParams holds inputs for finding the main worktree path.
type FindMainWorktreeParams struct {
	ProjectDir string
}

// FindMainWorktreePath returns the path of the main (first) worktree.
func FindMainWorktreePath(ctx context.Context, params FindMainWorktreeParams) (string, error) {
	cmd := Command(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = params.ProjectDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git worktree list: %w", err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			return strings.TrimPrefix(line, "worktree "), nil
		}
	}

	return "", fmt.Errorf("no worktree found in output")
}

// FindWorktreeByBranchParams holds inputs for finding a worktree by branch name.
type FindWorktreeByBranchParams struct {
	ProjectDir string
	Branch     string
}

// FindWorktreeByBranch returns the worktree matching the given branch name.
func FindWorktreeByBranch(ctx context.Context, params FindWorktreeByBranchParams) (domain.GitWorktree, error) {
	worktrees, err := ListWorktrees(ctx, ListWorktreesParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return domain.GitWorktree{}, err
	}

	for _, wt := range worktrees {
		if wt.Branch == params.Branch {
			return wt, nil
		}
	}

	return domain.GitWorktree{}, fmt.Errorf("%w: %s", domain.ErrWorktreeNotFound, params.Branch)
}
