package infra

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// TipParams holds inputs for resolving a ref to a short SHA.
type TipParams struct {
	WorktreePath string
	Ref          string
}

// Tip returns the short commit SHA a ref points to.
func Tip(ctx context.Context, params TipParams) (string, error) {
	cmd := Command(ctx, "git", "-C", params.WorktreePath, "rev-parse", "--short", params.Ref)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", params.Ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RebaseResult reports the outcome of a rebase attempt.
type RebaseResult struct {
	Conflicted bool
	Output     string
	// Files lists the unmerged paths captured when the rebase conflicts. It is
	// populated in both the abort and keep-in-progress paths (captured while the
	// rebase is stopped, before any abort).
	Files []string
	// Kept is true when a conflicting rebase was left in progress (KeepConflict)
	// instead of being aborted.
	Kept bool
}

// RebaseOntoParams holds inputs for a transplanting rebase.
type RebaseOntoParams struct {
	WorktreePath string
	NewBase      string
	Upstream     string
	Branch       string
	// KeepConflict leaves a conflicting rebase in progress in the worktree
	// (for manual resolution) instead of aborting it.
	KeepConflict bool
}

// RebaseOnto runs `git rebase --onto <NewBase> <Upstream> <Branch>`, replaying
// only the commits unique to Branch (Upstream..Branch) onto NewBase. This avoids
// re-applying the parent's already-moved commits.
//
// On a conflict the unmerged files are captured. By default the rebase is then
// aborted so the worktree is left clean; with KeepConflict it is left in
// progress for manual resolution. Either way the result reports Conflicted=true
// with a nil error. A non-conflict failure returns an error (after a best-effort
// abort).
func RebaseOnto(ctx context.Context, params RebaseOntoParams) (RebaseResult, error) {
	// The rebase may be interrupted, its abort may not: the process waits for
	// it, so no worktree is left half rebased behind an exit.
	settled, release := Shield(ctx)
	defer release()
	cmd := Command(ctx, "git", "-C", params.WorktreePath,
		"rebase", "--onto", params.NewBase, params.Upstream, params.Branch)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return RebaseResult{}, nil
	}

	output := strings.TrimSpace(string(out))

	if strings.Contains(output, "CONFLICT") {
		files := ConflictedFiles(ctx, params.WorktreePath)
		if params.KeepConflict {
			return RebaseResult{Conflicted: true, Output: output, Files: files, Kept: true}, nil
		}
		abortRebase(settled, params.WorktreePath)
		return RebaseResult{Conflicted: true, Output: output, Files: files}, nil
	}

	abortRebase(settled, params.WorktreePath)
	return RebaseResult{Output: output}, fmt.Errorf("git rebase: %s", output)
}

// ConflictedFiles returns the unmerged paths in a worktree, i.e. the files with
// conflict markers during a stopped rebase/merge (`git diff --name-only
// --diff-filter=U`). A clean tree or any error yields nil.
func ConflictedFiles(ctx context.Context, worktreePath string) []string {
	cmd := Command(ctx, "git", "-C", worktreePath, "diff", "--name-only", "--diff-filter=U")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// abortRebase best-effort aborts an in-progress rebase. A "no rebase in
// progress" error is expected when the failure happened before any apply.
func abortRebase(settled context.Context, worktreePath string) {
	_ = Command(settled, "git", "-C", worktreePath, "rebase", "--abort").Run()
}

// FastForwardParams holds inputs for fast-forwarding a checked-out branch.
type FastForwardParams struct {
	WorktreePath string
	Onto         string
}

// FastForwardBranch fast-forwards the branch checked out in WorktreePath to Onto
// (e.g. origin/main). It fails if the branch has diverged (no fast-forward).
func FastForwardBranch(ctx context.Context, params FastForwardParams) error {
	cmd := Command(ctx, "git", "-C", params.WorktreePath, "merge", "--ff-only", params.Onto)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git merge --ff-only %s: %s", params.Onto, strings.TrimSpace(string(out)))
	}
	return nil
}

// BehindParams holds inputs for counting how far a branch is behind an upstream.
type BehindParams struct {
	WorktreePath string
	Branch       string
	Upstream     string
}

// Behind returns the number of commits present in Upstream but not in Branch.
// Zero means the branch already contains everything from its parent.
func Behind(ctx context.Context, params BehindParams) (int, error) {
	return revListCount(ctx, params.WorktreePath, params.Branch+".."+params.Upstream)
}

// CommitCountParams holds inputs for counting commits in a revision range.
type CommitCountParams struct {
	WorktreePath string
	Range        string
}

// CommitCount returns the number of commits in the given revision range
// (e.g. "<oldParentTip>..<branch>").
func CommitCount(ctx context.Context, params CommitCountParams) (int, error) {
	return revListCount(ctx, params.WorktreePath, params.Range)
}

func revListCount(ctx context.Context, worktreePath, revRange string) (int, error) {
	cmd := Command(ctx, "git", "-C", worktreePath, "rev-list", "--count", revRange)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("git rev-list --count %s: %w", revRange, err)
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parse rev-list count %q: %w", strings.TrimSpace(string(out)), err)
	}
	return count, nil
}

// IsAncestorParams holds inputs for an ancestry check.
type IsAncestorParams struct {
	WorktreePath string
	Ancestor     string
	Descendant   string
}

// IsAncestor reports whether Ancestor is an ancestor of (or equal to) Descendant.
func IsAncestor(ctx context.Context, params IsAncestorParams) bool {
	cmd := Command(ctx, "git", "-C", params.WorktreePath,
		"merge-base", "--is-ancestor", params.Ancestor, params.Descendant)
	return cmd.Run() == nil
}

// RemoteBranchParams holds inputs for an operation comparing a branch to its
// own origin/<branch> remote-tracking ref.
type RemoteBranchParams struct {
	WorktreePath string
	Branch       string
}

// RemoteHasUnintegratedCommits reports whether origin/<Branch> carries commits
// that are NOT already present (by patch) in the local Branch. It runs
// `git cherry <Branch> origin/<Branch>`, which marks remote commits with "-" when
// their patch already exists locally (e.g. commits replayed by a prior rebase) and
// "+" when genuinely missing. Only "+" lines count as a real divergence. A missing
// remote ref or any error yields false (nothing to integrate). Reads local refs
// only — no network.
func RemoteHasUnintegratedCommits(ctx context.Context, params RemoteBranchParams) bool {
	cmd := Command(ctx, "git", "-C", params.WorktreePath,
		"cherry", params.Branch, "origin/"+params.Branch)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.HasPrefix(line, "+") {
			return true
		}
	}
	return false
}

// AheadOfRemote reports whether the local Branch has commits to force-push: true
// when origin/<Branch> does not exist (a push would create it) or when the local
// tip differs from the remote tip. Meant to be called only on non-diverged
// branches (local is ahead or rewritten, never behind unresolved). Reads local
// refs only — no network.
func AheadOfRemote(ctx context.Context, params RemoteBranchParams) bool {
	remoteRef := "origin/" + params.Branch
	remoteTip, err := Command(ctx, "git", "-C", params.WorktreePath, "rev-parse", remoteRef).Output()
	if err != nil {
		// Only treat the branch as pushable when origin/<branch> genuinely does
		// not exist (a push would create it). Any other rev-parse failure must
		// NOT be read as "remote missing" — that could trigger an unwanted
		// force-push under --push (non-interactive).
		return !remoteRefExists(ctx, params.WorktreePath, remoteRef)
	}
	localTip, err := Command(ctx, "git", "-C", params.WorktreePath, "rev-parse", params.Branch).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(localTip)) != strings.TrimSpace(string(remoteTip))
}

// remoteRefExists reports whether a remote-tracking ref (e.g. origin/<branch>)
// is present locally. Reads local refs only — no network.
func remoteRefExists(ctx context.Context, worktreePath, remoteRef string) bool {
	cmd := Command(ctx, "git", "-C", worktreePath,
		"show-ref", "--verify", "--quiet", "refs/remotes/"+remoteRef)
	return cmd.Run() == nil
}

// PushForceParams holds inputs for a force-with-lease push.
type PushForceParams struct {
	WorktreePath string
	Branch       string
}

// PushForceWithLease pushes Branch to origin with --force-with-lease, which
// refuses to overwrite remote commits the local ref has not seen.
func PushForceWithLease(ctx context.Context, params PushForceParams) error {
	cmd := Command(ctx, "git", "-C", params.WorktreePath,
		"push", "--force-with-lease", "origin", params.Branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push --force-with-lease origin %s: %s", params.Branch, strings.TrimSpace(string(out)))
	}
	return nil
}
