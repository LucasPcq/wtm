package infra

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// IsDirtyParams holds inputs for checking worktree dirty state.
type IsDirtyParams struct {
	WorktreePath string
}

// IsDirty checks if a worktree has uncommitted changes.
func IsDirty(ctx context.Context, params IsDirtyParams) (bool, error) {
	cmd := Command(ctx, "git", "-C", params.WorktreePath, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// CommitsAheadParams holds inputs for counting commits ahead.
type CommitsAheadParams struct {
	WorktreePath string
	BaseBranch   string
	Branch       string
}

// CommitsAhead returns how many commits a branch is ahead of the base branch.
// Returns 0 if the base branch doesn't exist or on error.
func CommitsAhead(ctx context.Context, params CommitsAheadParams) (int, error) {
	cmd := Command(ctx, "git", "-C", params.WorktreePath, "rev-list", "--count",
		params.BaseBranch+".."+params.Branch)
	out, err := cmd.Output()
	if err != nil {
		return 0, nil
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, nil
	}
	return count, nil
}

// ListModifiedFilesParams holds inputs for listing modified files.
type ListModifiedFilesParams struct {
	WorktreePath string
}

// ListModifiedFiles returns every uncommitted change of a worktree, one entry per
// file. -z keeps paths verbatim (no quoting of spaces or non-ASCII) and
// --untracked-files=all lists the contents of new directories instead of
// collapsing them into a single entry, so each file can be selected on its own.
func ListModifiedFiles(ctx context.Context, params ListModifiedFilesParams) ([]domain.PorcelainEntry, error) {
	cmd := Command(ctx, "git", "-C", params.WorktreePath,
		"status", "--porcelain", "-z", "--untracked-files=all")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}

	entries, err := rules.ParsePorcelainZ(out)
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}
	return entries, nil
}

// UnpushedCommitsParams holds inputs for counting unpushed commits.
type UnpushedCommitsParams struct {
	ProjectDir string
	Branch     string
}

// UnpushedCommits returns the count of local commits not present on the remote.
// Returns 0 if there is no remote tracking branch.
func UnpushedCommits(ctx context.Context, params UnpushedCommitsParams) (int, error) {
	cmd := Command(ctx, "git", "rev-list", "--count", "origin/"+params.Branch+".."+params.Branch)
	cmd.Dir = params.ProjectDir
	out, err := cmd.Output()
	if err != nil {
		return 0, nil
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, nil
	}
	return count, nil
}
