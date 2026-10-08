package infra

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// GitCommonDirParams holds inputs for resolving the git common dir.
type GitCommonDirParams struct {
	Dir string
}

// GitCommonDir runs `git rev-parse --git-common-dir` and resolves the result
// to an absolute path. The common dir is shared across all worktrees of a
// clone, so the returned path is stable from any worktree.
func GitCommonDir(ctx context.Context, params GitCommonDirParams) (string, error) {
	cmd := Command(ctx, "git", "rev-parse", "--git-common-dir")
	cmd.Dir = params.Dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir: %w", err)
	}

	path := strings.TrimSpace(string(out))
	if !filepath.IsAbs(path) {
		path = filepath.Join(params.Dir, path)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	return abs, nil
}

// Toplevel runs `git rev-parse --show-toplevel` and returns the root directory
// of the worktree containing dir.
func Toplevel(ctx context.Context, dir string) (string, error) {
	cmd := Command(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// InsideGitRepo reads git's own verdict rather than any failure of rev-parse:
// a missing git binary or an unreadable directory is an error, not an answer.
func InsideGitRepo(ctx context.Context, dir string) (bool, error) {
	cmd := Command(ctx, "git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && strings.Contains(stderr.String(), domain.GitNotARepoStderr) {
		return false, nil
	}
	return false, fmt.Errorf("git rev-parse --git-dir: %w", err)
}
