package infra

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CreateWorktreeParams holds inputs for creating a git worktree.
type CreateWorktreeParams struct {
	ProjectDir string
	Path       string
	Branch     string
	FromBranch string
	// ReuseBranch checks out an existing local branch instead of creating one
	// with -b; FromBranch is then irrelevant. The caller decides — it already
	// inspected the branch to reject one checked out elsewhere.
	ReuseBranch bool
}

// CreateWorktree creates a git worktree, either on a new branch (-b, from
// FromBranch) or on an existing local branch (ReuseBranch).
func CreateWorktree(params CreateWorktreeParams) error {
	if params.ReuseBranch {
		return createWorktreeExisting(params)
	}
	return createWorktreeNew(params)
}

func createWorktreeNew(params CreateWorktreeParams) error {
	cmd := exec.Command("git", "worktree", "add", "--no-track", "-b", params.Branch, params.Path, params.FromBranch)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func createWorktreeExisting(params CreateWorktreeParams) error {
	cmd := exec.Command("git", "worktree", "add", params.Path, params.Branch)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// MoveWorktreeParams holds inputs for moving a git worktree to a new path.
type MoveWorktreeParams struct {
	ProjectDir string
	From       string
	To         string
	Force      bool
}

// MoveWorktree relocates a git worktree directory via `git worktree move`. The
// parent directory of To must already exist. Force passes `--force` twice, which
// git requires to move a locked worktree (and is a harmless no-op otherwise).
func MoveWorktree(params MoveWorktreeParams) error {
	args := []string{"worktree", "move"}
	if params.Force {
		args = append(args, "--force", "--force")
	}
	args = append(args, params.From, params.To)

	cmd := exec.Command("git", args...)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree move: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// RemoveWorktreeParams holds inputs for removing a git worktree.
type RemoveWorktreeParams struct {
	ProjectDir string
	Path       string
	Force      bool
}

// RemoveWorktree removes a git worktree directory.
func RemoveWorktree(params RemoveWorktreeParams) error {
	args := []string{"worktree", "remove", params.Path}
	if params.Force {
		args = append(args, "--force")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// SudoDeleteDirParams holds inputs for a privileged directory deletion.
type SudoDeleteDirParams struct {
	Path string
}

// SudoDeleteDir removes a directory tree with `sudo rm -rf`. It inherits the
// current stdio so sudo can prompt for the password on the terminal — do not
// capture the output. Used as a last-resort fallback when `git worktree remove`
// cannot delete files owned by another user (e.g. root-owned Docker files).
func SudoDeleteDir(params SudoDeleteDirParams) error {
	cmd := exec.Command("sudo", "rm", "-rf", params.Path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo rm -rf %s: %w", params.Path, err)
	}
	return nil
}

// PruneWorktrees runs `git worktree prune` to clear the administrative metadata
// (.git/worktrees/<name>) left behind when a worktree directory is removed
// outside of git (e.g. by SudoDeleteDir).
func PruneWorktrees(projectDir string) error {
	cmd := exec.Command("git", "worktree", "prune")
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree prune: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
